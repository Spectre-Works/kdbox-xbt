package main

import (
	"bytes"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed web/*
var assets embed.FS

type Config struct {
	AdminPassword    string `json:"admin_password"`
	FallbackPassword string `json:"fallback_password,omitempty"`
}
type Pending struct {
	Previous  Tuning `json:"previous"`
	Candidate Tuning `json:"candidate"`
}
type Session struct{ Expires time.Time }
type Controller struct {
	firmware                                     string
	mu                                           sync.Mutex
	dir                                          string
	backend                                      string
	algorithm                                    int
	engineGrace                                  time.Time
	reload                                       func() error
	cfg                                          Config
	client                                       *http.Client
	token                                        string
	approved                                     Tuning
	pending                                      *Pending
	started, deadline, lastGood, boot            time.Time
	baselineHW, baselineNonces, baselineAccepted float64
	samples                                      int
	message                                      string
	halted                                       bool
	sessions                                     map[string]Session
	device                                       map[string]any
	networkPending                               *NetworkTrial
	recovering                                   bool
}

func atomicJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path+".new", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Rename(path+".new", path); err != nil {
		return err
	}
	if d, e := os.Open(filepath.Dir(path)); e == nil {
		defer d.Close()
		_ = d.Sync()
	}
	return nil
}
func (c *Controller) nativeOnce(method, path string, body any, auth bool) ([]byte, error) {
	var data []byte
	var err error
	if body != nil {
		data, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequest(method, c.backend+path, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if auth {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	res, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	b, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 {
		return nil, stockHTTPError(res.StatusCode)
	}
	return b, nil
}
func (c *Controller) login(password string) (string, error) {
	b, e := c.native("GET", "/user/login?username=admin&password="+url.QueryEscape(password), nil, false)
	if e != nil {
		return "", e
	}
	var v map[string]string
	if json.Unmarshal(b, &v) != nil || v["JWT Token"] == "" {
		return "", errors.New("login rejected")
	}
	return v["JWT Token"], nil
}
func (c *Controller) apply(t Tuning) error {
	token, err := c.login(c.cfg.AdminPassword)
	if err != nil {
		return err
	}
	c.token = token
	b, err := c.native("GET", "/mcb/setting", nil, false)
	if err != nil {
		return err
	}
	var v map[string]any
	if err = json.Unmarshal(b, &v); err != nil {
		return err
	}
	v["tempcontrol"] = true
	v["manual"] = false
	v["select"] = 0
	v["manualPowerplan"] = t.Powerplan()
	v["powerplans"] = []any{map[string]any{"level": 0, "info": t.Powerplan()}}
	_, err = c.native("PUT", "/mcb/setting", v, true)
	if err == nil {
		c.engineGrace = time.Now().Add(90 * time.Second)
		if c.reload != nil {
			err = c.reload()
		}
	}
	return err
}
func (c *Controller) rollback(reason string) error {
	if c.pending == nil {
		return nil
	}
	previous := c.pending.Previous
	if err := c.apply(previous); err != nil {
		if !c.recovering {
			c.stop("Rollback failed; mining stopped: " + err.Error())
		}
		return err
	}
	// Persist approved before clearing pending. An interrupted rollback retries safely.
	if err := atomicJSON(filepath.Join(c.dir, "approved.json"), previous); err != nil {
		if !c.recovering {
			c.stop("Cannot save rollback")
		}
		return err
	}
	if err := os.Remove(filepath.Join(c.dir, "pending.json")); err != nil {
		if !c.recovering {
			c.stop("Cannot clear pending trial")
		}
		return err
	}
	c.approved = previous
	c.pending = nil
	c.message = reason
	c.lastGood = time.Now()
	return nil
}
func (c *Controller) stop(reason string) {
	if !c.halted {
		_ = exec.Command("/bin/sh", "-c", `for p in $(pidof xminerd); do kill -STOP "$p"; done; for p in $(pidof intminer kdaminer); do kill -TERM "$p"; done; sleep 1; for p in $(pidof intminer kdaminer); do kill -KILL "$p"; done`).Run()
	}
	c.halted = true
	c.message = reason
	if saved, err := saveStop(filepath.Join(c.dir, "halted.json"), reason, c.device); err != nil {
		log.Print("Cannot persist safety stop: ", err)
	} else {
		c.message = saved
	}
	log.Print("Mining stopped: ", c.message)
	c.fans(100)
}
func (c *Controller) fans(level int) {
	for i := 0; i < 2; i++ {
		_ = os.WriteFile(fmt.Sprintf("/sys/class/fans/fan%d/lvl", i), []byte(strconv.Itoa(level)), 0600)
	}
}

var numbers = regexp.MustCompile(`[0-9]+(?:\.[0-9]+)?`)

func number(v any) float64 { f, _ := v.(float64); return f }
func temperature(v any) (float64, bool) {
	s, _ := v.(string)
	n := numbers.FindString(s)
	f, e := strconv.ParseFloat(n, 64)
	return f, e == nil && f > 0
}
func (c *Controller) safety() {
	// Keep cooling at full speed while the native engine initializes telemetry.
	if time.Now().Before(c.engineGrace) {
		c.fans(100)
	}
	b, err := c.native("GET", "/mcb/cgminer?cgminercmd=devs", nil, false)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.networkPending != nil && time.Now().After(c.networkPending.Deadline) {
		_ = c.restoreNetwork()
	}
	if c.pending != nil && time.Now().After(c.deadline) {
		_ = c.rollback("Trial expired; previous settings restored")
	}
	if c.halted {
		c.fans(100)
		return
	}
	var v struct {
		Status int              `json:"status"`
		Data   []map[string]any `json:"data"`
	}
	if err != nil || json.Unmarshal(b, &v) != nil || v.Status != 2 || len(v.Data) != 1 {
		if time.Now().After(c.engineGrace) && time.Since(c.lastGood) > 25*time.Second {
			if c.pending != nil {
				_ = c.rollback("Telemetry lost during trial")
			}
			c.stop("Mining telemetry unavailable; stopped safely")
		}
		return
	}
	d := v.Data[0]
	peak, peakErr := c.chipPeak()
	if peakErr == nil {
		d["peak_temp"] = peak
	}
	if live, e := liveMiner(); e == nil {
		d["clock"] = live["clock"]
		d["voltage"] = live["voltage"]
	}
	c.device = d
	temp, ok := temperature(d["temp"])
	if (!ok || peakErr != nil) && time.Now().Before(c.engineGrace) {
		return
	}
	averageLimit, peakLimit := thermalLimits(c.firmware)
	if !ok || peakErr != nil || temp >= averageLimit || number(d["peak_temp"]) >= peakLimit {
		if c.pending != nil {
			_ = c.rollback("Temperature fault")
		}
		c.stop(fmt.Sprintf("Temperature safety stop: average %.1f C (valid=%t, limit %.0f C), chip %.1f C (limit %.0f C), sensor error=%v", temp, ok, averageLimit, peak, peakLimit, peakErr))
		return
	}
	c.lastGood = time.Now()
	t := c.approved
	if c.pending != nil {
		t = c.pending.Candidate
	}
	c.fans(t.FanAt(temp))
	for i := 0; i < 2; i++ {
		readRPM := func() (int, error) {
			b, e := os.ReadFile(fmt.Sprintf("/sys/class/fans/fan%d/rpm", i))
			if e != nil {
				return 0, e
			}
			return strconv.Atoi(strings.TrimSpace(string(b)))
		}
		if !fanReady(readRPM, time.Now().Before(c.engineGrace), time.Sleep) {
			if c.pending != nil {
				_ = c.rollback("Fan RPM fault")
			}
			c.stop("Fan RPM below 3000; mining stopped")
			return
		}
	}
	if c.pending != nil && time.Now().After(c.engineGrace) {
		if number(d["clock"]) != float64(c.pending.Candidate.Clock) || int(number(d["voltage"])*1000+0.5) != c.pending.Candidate.Millivolts {
			_ = c.rollback("Live clock or voltage did not match the trial")
			return
		}
		if number(d["nonces"]) < c.baselineNonces || number(d["accepted"]) < c.baselineAccepted {
			c.baselineHW = 0
			c.baselineNonces = 0
			c.baselineAccepted = 0
		}
		c.samples++
		rate, valid := errorRate(c.baselineHW, c.baselineNonces, number(d["hwerrors"]), number(d["nonces"]))
		if temp >= 82 || number(d["peak_temp"]) >= 88 || (valid && number(d["nonces"])-c.baselineNonces >= 500 && rate > 1) {
			_ = c.rollback("Unstable trial; previous settings restored")
		}
	}
}
func jsonReply(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func decode(r *http.Request, v any) error {
	d := json.NewDecoder(io.LimitReader(r.Body, 8193))
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return e
	}
	if d.Decode(&struct{}{}) != io.EOF {
		return errors.New("invalid request body")
	}
	return nil
}
func (c *Controller) handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	origin := r.Header.Get("Origin")
	if origin != "" {
		if !allowedOrigin(origin, r.Host) {
			jsonReply(w, 403, map[string]string{"error": "origin rejected"})
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Vary", "Origin")
	}
	if r.Method == "OPTIONS" {
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(204)
		return
	}
	if r.Method != "GET" {
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			jsonReply(w, 415, map[string]string{"error": "JSON required"})
			return
		}
	}
	if r.URL.Path == "/api/login" && r.Method == "POST" {
		var q struct {
			Password string `json:"password"`
		}
		if decode(r, &q) != nil {
			http.Error(w, "bad request", 400)
			return
		}
		if _, e := c.login(q.Password); e != nil {
			jsonReply(w, 401, map[string]string{"error": "Miner password not accepted"})
			return
		}
		key := make([]byte, 32)
		if _, e := rand.Read(key); e != nil {
			http.Error(w, "random source unavailable", 500)
			return
		}
		id := hex.EncodeToString(key)
		c.mu.Lock()
		for k, s := range c.sessions {
			if time.Now().After(s.Expires) {
				delete(c.sessions, k)
			}
		}
		if len(c.sessions) >= 100 {
			c.mu.Unlock()
			http.Error(w, "too many sessions", 429)
			return
		}
		c.sessions[id] = Session{time.Now().Add(30 * time.Minute)}
		c.mu.Unlock()
		http.SetCookie(w, &http.Cookie{Name: "paperclip", Value: id, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: 1800})
		jsonReply(w, 200, map[string]bool{"ok": true})
		return
	}
	if r.Method == "GET" && r.URL.Path == "/api/history" {
		b, e := c.native("GET", "/cpb/hshistory", nil, false)
		if e != nil {
			jsonReply(w, 503, map[string]string{"error": "history unavailable"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(b)
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if r.Method == "GET" && r.URL.Path == "/api/status" {
		jsonReply(w, 200, map[string]any{"algorithm": c.algorithm, "device": c.device, "approved": c.approved, "pending": c.pending, "seconds_left": max(0, int(time.Until(c.deadline).Seconds())), "message": c.message, "halted": c.halted, "stale": time.Since(c.lastGood) > 15*time.Second})
		return
	}
	cookie, e := r.Cookie("paperclip")
	if r.Method == "GET" && r.URL.Path == "/api/pool-status" {
		b, err := c.native("GET", "/mcb/pools", nil, false)
		if err != nil {
			jsonReply(w, 502, map[string]string{"error": "Pool settings unavailable"})
			return
		}
		pools, err := publicPools(poolTelemetry(b))
		if err != nil {
			jsonReply(w, 502, map[string]string{"error": "Invalid pool settings"})
			return
		}
		editable := e == nil && time.Now().Before(c.sessions[cookie.Value].Expires)
		jsonReply(w, 200, map[string]any{"pools": pools, "editable": editable})
		return
	}
	if e != nil || time.Now().After(c.sessions[cookie.Value].Expires) {
		jsonReply(w, 401, map[string]string{"error": "Unlock with your miner password first"})
		return
	}
	if c.settings(w, r) {
		return
	}
	var err error
	switch {
	case r.URL.Path == "/api/trial" && r.Method == "POST":
		var t Tuning
		err = decode(r, &t)
		if err == nil {
			err = t.Validate(c.approved)
		}
		if err == nil && (c.pending != nil || c.halted || c.device == nil || number(c.device["nonces"]) < 1 || time.Since(c.lastGood) > 10*time.Second) {
			err = errors.New("healthy miner with no pending trial required")
		}
		if err == nil {
			p := Pending{c.approved, t}
			err = atomicJSON(filepath.Join(c.dir, "pending.json"), p)
			if err == nil {
				c.pending = &p
				c.started = time.Now()
				c.deadline = c.started.Add(3 * time.Minute)
				c.samples = 0
				c.baselineHW = number(c.device["hwerrors"])
				c.baselineNonces = number(c.device["nonces"])
				c.baselineAccepted = number(c.device["accepted"])
				err = c.apply(t)
				if err != nil {
					_ = c.rollback("Trial apply failed")
				} else {
					c.message = "Trial running; confirm after healthy mining or it rolls back"
				}
			}
		}
	case r.URL.Path == "/api/confirm" && r.Method == "POST":
		if c.pending == nil || !time.Now().Before(c.deadline) || time.Now().Before(c.engineGrace) || time.Since(c.started) < 60*time.Second || c.samples < 10 || time.Since(c.lastGood) > 10*time.Second || number(c.device["accepted"]) <= c.baselineAccepted {
			err = errors.New("wait for 60 seconds of healthy mining and a new accepted share")
		} else {
			err = atomicJSON(filepath.Join(c.dir, "approved.json"), c.pending.Candidate)
			if err == nil {
				err = os.Remove(filepath.Join(c.dir, "pending.json"))
				if err == nil {
					c.approved = c.pending.Candidate
					c.pending = nil
					c.message = "Settings confirmed"
				}
			}
		}
	case r.URL.Path == "/api/resume" && r.Method == "POST":
		if c.pending != nil {
			err = errors.New("restart controller to recover pending trial first")
		} else if c.halted {
			err = c.apply(c.approved)
			if err == nil {
				err = os.Remove(filepath.Join(c.dir, "halted.json"))
				if err == nil {
					c.halted = false
					c.lastGood = time.Now()
					c.message = "Confirmed settings restored"
					_ = exec.Command("/bin/sh", "-c", "kill -CONT $(pidof xminerd)").Run()
					err = c.reapplyAlgorithm()
					if err != nil {
						c.stop("Resume failed while restoring mining engine: " + err.Error())
					}
				}
			}
		}
	case r.URL.Path == "/api/revert" && r.Method == "POST":
		err = c.rollback("Previous settings restored")
	case r.URL.Path == "/api/algorithm" && r.Method == "POST":
		var q struct {
			ID int `json:"id"`
		}
		err = decode(r, &q)
		if err == nil && (q.ID < 0 || q.ID > 1 || c.pending != nil || c.halted) {
			err = errors.New("invalid algorithm or active trial/fault")
		}
		if err == nil {
			var b []byte
			b, err = c.native("GET", "/mcb/algosetting", nil, false)
			var v map[string]any
			if err == nil {
				err = json.Unmarshal(b, &v)
			}
			if err == nil {
				v["algo_select"] = q.ID
				c.token, err = c.login(c.cfg.AdminPassword)
				if err == nil {
					_, err = c.native("PUT", "/mcb/algosetting", v, true)
					c.algorithm = q.ID
					c.boot = time.Now()
					c.engineGrace = c.boot.Add(90 * time.Second)
					c.lastGood = c.boot
					c.device = nil
					c.message = "Algorithm changed; update your pool for that algorithm"
				}
			}
		}
	default:
		jsonReply(w, 404, map[string]string{"error": "not found"})
		return
	}
	if err != nil {
		jsonReply(w, 400, map[string]string{"error": err.Error()})
	} else {
		jsonReply(w, 200, map[string]bool{"ok": true})
	}
}
func main() {
	dir := flag.String("state", "/usr/config/paperclip", "private state directory")
	preflight := flag.Bool("preflight", false, "recover disk parameters before native mining startup")
	listen := flag.String("listen", ":8088", "listen address")
	flag.Parse()
	if *preflight {
		if e := diskRecover(*dir, "/usr/config/minerd/settings", "/usr/config/bfgminer"); e != nil {
			log.Fatal(e)
		}
		return
	}
	c := &Controller{dir: *dir, backend: "http://127.0.0.1", engineGrace: time.Now().Add(180 * time.Second), client: &http.Client{Timeout: 4 * time.Second}, boot: time.Now(), lastGood: time.Now(), sessions: map[string]Session{}, message: "Starting"}
	c.reload = func() error {
		return exec.Command("/bin/sh", "-c", "for p in $(pidof intminer kdaminer); do kill -TERM \"$p\"; done; true").Run()
	}
	b, e := os.ReadFile(filepath.Join(*dir, "config.json"))
	if e != nil || json.Unmarshal(b, &c.cfg) != nil || c.cfg.AdminPassword == "" {
		log.Fatal("missing valid private config")
	}
	b, e = os.ReadFile(filepath.Join(*dir, "approved.json"))
	if e == nil {
		e = json.Unmarshal(b, &c.approved)
		if c.approved.Millivolts == 0 {
			c.approved.Millivolts = 430
		}
	}
	if e != nil || c.approved.Validate(c.approved) != nil {
		log.Fatal("invalid approved settings")
	}
	if b, e = os.ReadFile(filepath.Join(*dir, "pending.json")); e == nil {
		var p Pending
		parseErr := json.Unmarshal(b, &p)
		if p.Previous.Millivolts == 0 {
			p.Previous.Millivolts = 430
		}
		if p.Candidate.Millivolts == 0 {
			p.Candidate.Millivolts = 430
		}
		if parseErr != nil || p.Previous.Validate(p.Previous) != nil {
			log.Fatal("invalid pending rollback record")
		}
		c.pending = &p
	}
	ready := false
	for i := 0; i < 45; i++ {
		b, e = c.native("GET", "/mcb/status", nil, false)
		var id map[string]any
		if e == nil && json.Unmarshal(b, &id) == nil {
			if _, exists := id["mcbversion"]; !exists {
				board, err := os.ReadFile("/root/mcbversion")
				if err != nil {
					log.Fatal("cannot read board identity")
				}
				id["mcbversion"] = strings.TrimSpace(string(board))
			}
			if !supportedDevice(id) {
				log.Fatal("unsupported device")
			}
			c.firmware, _ = id["firmware"].(string)
			if _, e = c.login(c.cfg.AdminPassword); e == nil || c.cfg.FallbackPassword != "" {
				b, e = c.native("GET", "/mcb/setting", nil, false)
				var settings map[string]any
				if e == nil && json.Unmarshal(b, &settings) == nil && settings["manualPowerplan"] != nil {
					ready = true
					break
				}
			}
		}
		time.Sleep(2 * time.Second)
	}
	if !ready {
		log.Fatal("stock controller not ready")
	}
	if c.cfg.FallbackPassword != "" {
		if _, e = c.login(c.cfg.AdminPassword); e != nil {
			if _, e = c.login(c.cfg.FallbackPassword); e != nil {
				log.Fatal("cannot recover account credentials")
			}
			c.cfg.AdminPassword = c.cfg.FallbackPassword
		}
		c.cfg.FallbackPassword = ""
		if e = atomicJSON(filepath.Join(*dir, "config.json"), c.cfg); e != nil {
			log.Fatal("cannot persist account recovery")
		}
	}
	if b, e = os.ReadFile(filepath.Join(*dir, "network-pending.json")); e == nil {
		var p NetworkTrial
		if json.Unmarshal(b, &p) != nil {
			log.Fatal("invalid network recovery record")
		}
		c.networkPending = &p
		if e = c.restoreNetwork(); e != nil {
			log.Fatal("network rollback failed")
		}
	}
	if c.pending != nil {
		c.recovering = true
		for i := 0; i < 15; i++ {
			e = c.rollback("Interrupted trial recovered at startup")
			if e == nil {
				break
			}
			log.Print("Waiting for recovery API: ", e)
			time.Sleep(2 * time.Second)
		}
		c.recovering = false
		if e != nil {
			c.stop("Boot rollback failed; mining held off")
			log.Fatal("pending rollback failed: ", e)
		}
	}
	if _, e = os.Stat(filepath.Join(*dir, "halted.json")); e == nil {
		c.stop("Safety stop retained; original cause unavailable. Check diagnostics before resuming")
	} else if e = c.apply(c.approved); e != nil {
		log.Fatal("cannot apply approved settings")
	}
	if b, e = c.native("GET", "/mcb/algosetting", nil, false); e == nil {
		var a struct {
			ID int `json:"algo_select"`
		}
		if json.Unmarshal(b, &a) == nil {
			c.algorithm = a.ID
			if !c.halted {
				if _, e = c.native("PUT", "/mcb/algosetting", json.RawMessage(b), true); e != nil {
					log.Fatal("cannot restore native algorithm")
				}
			}
		}
	}
	c.engineGrace = time.Now().Add(180 * time.Second)
	if e = os.WriteFile("/tmp/paperclip-ready", []byte("ready"), 0600); e != nil {
		log.Fatal(e)
	}
	if !c.halted {
		if c.message == "Starting" {
			c.message = "Paperclip safeguards active"
		}
		_ = exec.Command("/bin/sh", "-c", "kill -CONT $(pidof xminerd) 2>/dev/null").Run()
	}
	go func() {
		for {
			c.safety()
			_ = os.WriteFile("/tmp/paperclip-heartbeat", []byte(strconv.FormatInt(time.Now().Unix(), 10)), 0600)
			time.Sleep(3 * time.Second)
		}
	}()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/", c.handler)
	web, _ := fs.Sub(assets, "web")
	mux.Handle("/", http.FileServer(http.FS(web)))
	server := http.Server{Addr: *listen, Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 8192}
	log.Fatal(server.ListenAndServe())
}

// chipPeak authenticates when required by newer stock firmware and refreshes
// expired sessions before treating chip telemetry as unavailable.
func (c *Controller) chipPeak() (float64, error) {
	b, err := c.native("GET", "/dbg/icinfo", nil, true)
	if err != nil {
		token, loginErr := c.login(c.cfg.AdminPassword)
		if loginErr != nil {
			return 0, loginErr
		}
		c.token = token
		b, err = c.native("GET", "/dbg/icinfo", nil, true)
		if err != nil {
			return 0, err
		}
	}
	var outer struct {
		Body string `json:"body"`
	}
	var inner struct {
		Chips [][]map[string]any `json:"drawdata"`
	}
	if json.Unmarshal(b, &outer) != nil || json.Unmarshal([]byte(outer.Body), &inner) != nil {
		return 0, errors.New("invalid chip telemetry")
	}
	peak := 0.0
	count := 0
	for _, board := range inner.Chips {
		for _, chip := range board {
			temp, ok := chip["temp"].(float64)
			if !ok || temp <= 0 || temp > 200 {
				return 0, errors.New("missing chip temperature")
			}
			peak = max(peak, temp)
			count++
		}
	}
	if count == 0 {
		return 0, errors.New("empty chip telemetry")
	}
	return peak, nil
}
