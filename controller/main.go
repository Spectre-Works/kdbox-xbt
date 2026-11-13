package main

import (
	"bytes"
	"context"
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
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

//go:embed web/*
var assets embed.FS

type Config struct {
	AdminPassword    string             `json:"admin_password"`
	FallbackPassword string             `json:"fallback_password,omitempty"`
	ChipTarget       int                `json:"chip_target,omitempty"`
	AutomaticTuning  bool               `json:"automatic_tuning,omitempty"`
	AutomaticLimits  *performanceLimits `json:"automatic_limits,omitempty"`
}

func (cfg Config) coolingTarget() int {
	if cfg.ChipTarget == 0 {
		return chipTemperatureTarget
	}
	return cfg.ChipTarget
}

type Pending struct {
	Previous  Tuning `json:"previous"`
	Candidate Tuning `json:"candidate"`
}
type Session struct{ Expires time.Time }
type Controller struct {
	firmware                                     string
	mu                                           sync.Mutex
	diagnosticMu                                 sync.Mutex
	dir                                          string
	backend                                      string
	algorithm                                    int
	algorithmQueued                              bool
	engineGrace                                  time.Time
	startup                                      engineStartup
	cooling                                      automaticCooling
	history                                      *hashrateHistory
	miningSession                                miningSession
	automatic                                    automaticPerformance
	lastHardware                                 time.Time
	reload                                       func() error
	setClock                                     func(int) error
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
	return c.nativeOnceContext(context.Background(), method, path, body, auth)
}
func (c *Controller) nativeOnceContext(ctx context.Context, method, path string, body any, auth bool) ([]byte, error) {
	var data []byte
	var err error
	if body != nil {
		data, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, c.backend+path, bytes.NewReader(data))
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
	b, err := io.ReadAll(io.LimitReader(res.Body, (1<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(b) > 1<<20 {
		return nil, errors.New("stock response exceeded the 1 MiB read limit")
	}
	if res.StatusCode != 200 {
		return nil, stockHTTPError(res.StatusCode)
	}
	return b, nil
}
func (c *Controller) login(password string) (string, error) {
	return c.loginContext(context.Background(), password)
}
func (c *Controller) loginContext(ctx context.Context, password string) (string, error) {
	b, e := c.nativeOnceContext(ctx, "GET", "/user/login?username=admin&password="+url.QueryEscape(password), nil, false)
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
		c.cooling = automaticCooling{}
		c.engineGrace = time.Now().Add(90 * time.Second)
		if c.reload != nil {
			err = c.reload()
		}
		if err == nil && !c.halted {
			c.beginStockStartup(time.Now())
		}
		c.automatic = automaticPerformance{}
		c.lastHardware = time.Time{}
	}
	return err
}
func (c *Controller) rollback(reason string) error {
	if c.pending == nil {
		return nil
	}
	previous := initialPerformancePoint(c.cfg, c.pending.Previous)
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
			c.stop("Cannot clear pending performance adjustment")
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
	c.miningSession = miningSession{}
	c.cooling = automaticCooling{}
	c.startup = engineStartup{}
	c.message = reason
	if saved, err := saveStop(filepath.Join(c.dir, "halted.json"), reason, c.device); err != nil {
		log.Print("Cannot persist safety stop: ", err)
	} else {
		c.message = saved
	}
	log.Print("Mining stopped: ", c.message)
	c.fans(100)
}
func (c *Controller) fans(level int) error {
	var err error
	for i := 0; i < 2; i++ {
		err = errors.Join(err, writeFanPercent(i, level))
	}
	return err
}

var numbers = regexp.MustCompile(`-?[0-9]+(?:\.[0-9]+)?`)

func number(v any) float64 { f, _ := v.(float64); return f }
func temperature(v any) (float64, bool) {
	s, _ := v.(string)
	n := numbers.FindString(s)
	f, e := strconv.ParseFloat(n, 64)
	return f, e == nil && f > 0
}

func boardTemperature(v any) (float64, bool) {
	s, _ := v.(string)
	_, board, found := strings.Cut(s, "/")
	if !found {
		return 0, false
	}
	return temperature(board)
}
func (c *Controller) safety() {
	c.mu.Lock()
	defer func() {
		c.updateMiningSession(time.Now())
		c.mu.Unlock()
	}()
	if c.networkPending != nil && time.Now().After(c.networkPending.Deadline) {
		_ = c.restoreNetwork()
	}
	if c.halted {
		c.fans(100)
		return
	}
	if c.pending != nil && !c.deadline.IsZero() && time.Now().After(c.deadline) {
		c.rejectPerformance("Automatic settings could not be verified; restored previous settings")
		if c.halted {
			return
		}
	}
	if !c.startup.calibrationUntil.IsZero() {
		engineRunning, err := processRunning("intminer", "kdaminer")
		if err != nil {
			c.stop("Cannot verify that mining is off during stock fan calibration")
			return
		}
		waiting, err := c.startup.advance(time.Now(), engineRunning)
		if err != nil {
			c.stop(err.Error())
			return
		}
		if waiting {
			supervisorRunning, err := processRunning("xminerd")
			if err != nil {
				c.stop("Cannot verify the stock mining supervisor during fan calibration")
			} else if !supervisorRunning {
				c.fans(100)
			}
			return
		}
		c.message = "XBT engine started; waiting for fresh temperatures"
	}
	waitingForReadings := !c.startup.readingsUntil.IsZero()
	graceUntil := c.engineGrace
	if waitingForReadings {
		graceUntil = c.startup.readingsUntil
	}
	warming := time.Now().Before(graceUntil)
	if warming || waitingForReadings {
		c.cooling = automaticCooling{}
		c.fans(100)
	}
	if waitingForReadings && !c.checkFans(warming, 100) {
		return
	}
	b, err := c.native("GET", "/mcb/cgminer?cgminercmd=devs", nil, false)
	var v struct {
		Status int              `json:"status"`
		Data   []map[string]any `json:"data"`
	}
	if err != nil || json.Unmarshal(b, &v) != nil || v.Status != 2 || len(v.Data) != 1 {
		c.cooling = automaticCooling{}
		c.fans(100)
		if !time.Now().Before(graceUntil) && time.Since(c.lastGood) > 25*time.Second {
			c.stop("Mining telemetry unavailable; stopped safely")
		}
		return
	}
	d := v.Data[0]
	peak, peakErr := c.chipPeak()
	if peakErr == nil {
		d["peak_temp"] = peak
	}
	hardwareVerified := false
	if live, e := liveMiner(); e == nil {
		hardwareVerified = number(live["clock"]) > 0 && number(live["voltage"]) > 0
		d["clock"] = live["clock"]
		d["voltage"] = live["voltage"]
	}
	c.device = d
	temp, ok := temperature(d["temp"])
	board, boardOK := boardTemperature(d["temp"])
	if ok {
		d["chip_average"] = temp
	}
	if boardOK {
		d["board_temp"] = board
	}
	averageLimit, peakLimit := thermalLimits(c.firmware)
	overTemperature := (ok && temp >= averageLimit) || peak >= peakLimit || (boardOK && board >= 86)
	if !overTemperature && (!ok || !boardOK || peakErr != nil) && time.Now().Before(graceUntil) {
		return
	}
	if !ok || !boardOK || peakErr != nil || overTemperature {
		chipReading := fmt.Sprintf("%.1f C", peak)
		if peakErr != nil && peak == 0 {
			chipReading = "unavailable"
		}
		c.stop(fmt.Sprintf("Temperature safety stop: average %.1f C (valid=%t, limit %.0f C), board %.1f C (valid=%t, limit 86 C), chip %s (limit %.0f C), sensor error=%v", temp, ok, averageLimit, board, boardOK, chipReading, peakLimit, peakErr))
		return
	}
	if waitingForReadings {
		c.startup = engineStartup{}
		c.engineGrace = time.Time{}
		c.message = "KD BOX safeguards active"
	}
	c.lastGood = time.Now()
	previousPercent := c.cooling.percent
	percent := c.cooling.next(time.Now(), board, temp, peak, float64(c.cfg.coolingTarget()), averageLimit, peakLimit)
	if c.cfg.AutomaticTuning {
		percent = 100
		c.cooling.percent = 100
	}
	if err := c.fans(percent); err != nil {
		c.stop("Cannot set automatic fan output; mining stopped")
		return
	}
	if !c.checkFans(time.Now().Before(c.engineGrace) || percent > previousPercent, percent) {
		return
	}
	c.performanceSafety(time.Now(), hardwareVerified)
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
		http.SetCookie(w, &http.Cookie{Name: "kdbox-xbt", Value: id, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: 1800})
		jsonReply(w, 200, map[string]bool{"ok": true})
		return
	}
	if r.Method == "GET" && r.URL.Path == "/api/history" {
		if c.history == nil {
			jsonReply(w, 503, map[string]string{"error": "history unavailable"})
			return
		}
		jsonReply(w, 200, map[string]any{"interval_seconds": 60, "samples": c.history.snapshot(time.Now())})
		return
	}
	c.mu.Lock()
	cookie, e := r.Cookie("kdbox-xbt")
	unlocked := e == nil && time.Now().Before(c.sessions[cookie.Value].Expires)
	if r.Method == "GET" && r.URL.Path == "/api/diagnostics" {
		halted, password := c.halted, c.cfg.AdminPassword
		c.mu.Unlock()
		c.diagnostics(r.Context(), w, r.URL.Query().Get("kind"), unlocked, halted, password)
		return
	}
	if r.Method == "GET" && r.URL.Path == "/api/status" {
		now := time.Now()
		c.updateMiningSession(now)
		status := map[string]any{
			"software":  softwareInfo(),
			"algorithm": c.algorithm, "algorithm_queued": c.algorithmQueued,
			"startup":        c.startup.phase(),
			"mining_session": c.miningSession.status(now),
			"fan_rpm":        fanReadings(), "device": c.device, "approved": c.approved,
			"fan_percent": fanPercentages(),
			"cooling": map[string]any{"target": c.cfg.coolingTarget(), "minimum_percent": minimumFanPercent,
				"filtered_peak": c.cooling.peak, "projected_peak": c.cooling.projected[2],
				"rise_c_per_minute": max(0, c.cooling.trend[2]) * 60, "headroom_c": c.cooling.headroom,
				"boosted": !c.cooling.boostUntil.IsZero()},
			"performance": c.performanceStatus(),
			"message":     c.message, "halted": c.halted, "stale": now.Sub(c.lastGood) > 15*time.Second,
			"controls_unlocked": unlocked,
		}
		c.mu.Unlock()
		jsonReply(w, 200, status)
		return
	}
	if r.Method == "GET" && r.URL.Path == "/api/pool-status" {
		c.mu.Unlock()
		b, err := c.nativeContext(r.Context(), "GET", "/mcb/pools", nil, false)
		if err != nil {
			jsonReply(w, 502, map[string]string{"error": "Pool settings unavailable"})
			return
		}
		pools, err := publicPools(poolTelemetry(b))
		if err != nil {
			jsonReply(w, 502, map[string]string{"error": "Invalid pool settings"})
			return
		}
		jsonReply(w, 200, map[string]any{"pools": pools, "editable": unlocked})
		return
	}
	if unlocked && r.Method == "GET" {
		if r.URL.Path == "/api/network-trial" {
			pending := c.networkPending
			c.mu.Unlock()
			jsonReply(w, 200, pending)
			return
		}
		c.mu.Unlock()
		if !c.settings(w, r) {
			jsonReply(w, 404, map[string]string{"error": "Unknown endpoint"})
		}
		return
	}
	defer c.mu.Unlock()
	if !unlocked {
		jsonReply(w, 401, map[string]string{"error": "Unlock with your miner password first"})
		return
	}
	if c.settings(w, r) {
		return
	}
	var err error
	switch {
	case r.URL.Path == "/api/cooling" && r.Method == "POST":
		var q struct {
			Target int `json:"target"`
		}
		err = decode(r, &q)
		if err == nil && (q.Target < 70 || q.Target > 80) {
			err = errors.New("hottest-chip limit must be 70–80 C")
		}
		if err == nil {
			cfg := c.cfg
			cfg.ChipTarget = q.Target
			err = atomicJSON(filepath.Join(c.dir, "config.json"), cfg)
			if err == nil {
				c.cfg = cfg
				if c.cooling.target != float64(q.Target) {
					c.cooling.retarget(float64(q.Target))
					c.automatic.hotSince, c.automatic.rawHotSince, c.automatic.coolSince = time.Time{}, time.Time{}, time.Time{}
				}
			}
		}
	case r.URL.Path == "/api/performance" && r.Method == "POST":
		var q struct {
			Automatic *bool `json:"automatic"`
		}
		err = decode(r, &q)
		if err == nil && q.Automatic == nil {
			err = errors.New("select automatic performance or SPECTRE defaults")
		}
		if err == nil {
			err = c.setAutomaticPerformance(*q.Automatic)
		}
	case r.URL.Path == "/api/resume" && r.Method == "POST":
		var pools []map[string]any
		pools, err = c.savedPools()
		if err == nil && len(pools) == 0 {
			err = errors.New("add a mining pool before resuming")
		}
		if err == nil {
			err = c.resumeMining()
		}
	case r.URL.Path == "/api/algorithm" && r.Method == "POST":
		var q struct {
			ID int `json:"id"`
		}
		err = decode(r, &q)
		if err == nil {
			err = c.chooseAlgorithm(q.ID)
		}
		if err == nil && !c.halted {
			c.startup = engineStartup{}
			c.boot = time.Now()
			c.engineGrace = c.boot.Add(90 * time.Second)
			c.lastGood = c.boot
			c.device = nil
			c.message = "Mining engine restarting on the selected algorithm"
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
	dir := flag.String("state", "/usr/config/kdbox-xbt", "private state directory")
	preflight := flag.Bool("preflight", false, "recover disk parameters before native mining startup")
	listen := flag.String("listen", ":8088", "listen address")
	flag.Parse()
	logPath := "/tmp/kdbox-xbt-controller.log"
	if *preflight {
		logPath = "/tmp/kdbox-xbt-preflight.log"
	}
	logger := &boundedLog{path: logPath, maximum: 256 << 10}
	_, _ = logger.Write(nil) // Trim an oversized log left by an older build too.
	log.SetOutput(logger)
	if e := provisioningReady(*dir); e != nil {
		log.Fatal(e)
	}
	if *preflight {
		if e := preflightDevice("/root"); e != nil {
			log.Fatal(e)
		}
		if e := diskRecover(*dir, "/usr/config/minerd/settings", "/usr/config/bfgminer"); e != nil {
			log.Fatal(e)
		}
		return
	}
	c := &Controller{dir: *dir, backend: "http://127.0.0.1", engineGrace: time.Now().Add(180 * time.Second), client: &http.Client{Timeout: 4 * time.Second}, boot: time.Now(), lastGood: time.Now(), sessions: map[string]Session{}, message: "Starting"}
	c.reload = restartMiningEngine
	c.setClock = setMinerClock
	b, e := os.ReadFile(filepath.Join(*dir, "config.json"))
	if e != nil || json.Unmarshal(b, &c.cfg) != nil || c.cfg.AdminPassword == "" {
		log.Fatal("missing valid private config")
	}
	if c.cfg.coolingTarget() < 70 || c.cfg.coolingTarget() > 80 || c.cfg.validatePerformance() != nil {
		log.Fatal("invalid saved control settings")
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
	if c.cfg.AutomaticTuning && c.firmware != "2.2.1" {
		c.stop("Automatic performance requires the verified 2.2.1 base")
		log.Fatal("unsupported automatic performance base")
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
			e = c.rollback("Interrupted automatic adjustment recovered at startup")
			if e == nil {
				break
			}
			log.Print("Waiting for recovery API: ", e)
			time.Sleep(2 * time.Second)
		}
		c.recovering = false
		if e != nil {
			c.stop("Boot rollback failed; mining held off")
			log.Fatal("performance recovery failed: ", e)
		}
	}
	if initial := initialPerformancePoint(c.cfg, c.approved); initial != c.approved {
		c.approved = initial
		if e = atomicJSON(filepath.Join(*dir, "approved.json"), c.approved); e != nil {
			log.Fatal("cannot save SPECTRE defaults")
		}
	}
	if _, e = os.Stat(filepath.Join(*dir, "halted.json")); e == nil {
		c.stop("Safety stop retained; original cause unavailable. Check diagnostics before resuming")
	} else if e = c.apply(c.approved); e != nil {
		log.Fatal("cannot apply approved settings")
	}
	if e = c.restoreAlgorithm(); e != nil {
		c.stop("Could not verify the selected mining algorithm: " + e.Error())
	}
	if !c.halted {
		pools, poolErr := c.savedPools()
		if poolErr != nil {
			c.stop("Could not verify configured mining pools")
		} else if len(pools) == 0 {
			c.stop(poolSetupPause)
		}
	}
	if c.cfg.AutomaticTuning && c.algorithm != 1 {
		c.stop("Automatic performance requires XBT; turn it off before selecting another algorithm")
	}
	c.engineGrace = time.Now().Add(180 * time.Second)
	if e = os.WriteFile("/tmp/kdbox-xbt-ready", []byte("ready"), 0600); e != nil {
		log.Fatal(e)
	}
	if !c.halted {
		if c.message == "Starting" {
			c.message = "KD BOX safeguards active"
		}
		c.beginStockStartup(time.Now())
		_ = exec.Command("/bin/sh", "-c", "kill -CONT $(pidof xminerd) 2>/dev/null").Run()
	}
	factoryReload := make(chan os.Signal, 1)
	signal.Notify(factoryReload, syscall.SIGHUP)
	go func() {
		for range factoryReload {
			c.mu.Lock()
			if err := c.reloadFactoryClockLimit(); err != nil {
				log.Print("Factory clock limit request rejected: ", err)
			}
			c.mu.Unlock()
		}
	}()
	c.history = &hashrateHistory{path: "/tmp/kdbox-xbt-history.json"}
	c.history.load(time.Now())
	go func() {
		for {
			b, err := c.native("GET", "/cpb/hshistory", nil, false)
			var values []float64
			if err == nil && json.Unmarshal(b, &values) == nil {
				if err := c.history.merge(time.Now(), values); err != nil {
					log.Print("Cannot preserve hashrate history: ", err)
				}
			}
			time.Sleep(time.Minute)
		}
	}()
	go func() {
		for {
			c.safety()
			_ = writeHeartbeat("/tmp/kdbox-xbt-heartbeat", time.Now())
			time.Sleep(3 * time.Second)
		}
	}()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/", c.handler)
	web, _ := fs.Sub(assets, "web")
	mux.Handle("/", http.FileServer(http.FS(web)))
	server := http.Server{Addr: *listen, Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 8192}
	listener, err := net.Listen("tcp", *listen)
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(server.Serve(newBoundedListener(listener, 32)))
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
	chipCount := 0
	var invalid error
	for boardIndex, board := range inner.Chips {
		for chipIndex, chip := range board {
			chipCount++
			temp, ok := chip["temp"].(float64)
			if ok && temp > 0 && temp <= 200 {
				peak = max(peak, temp)
				continue
			}
			if invalid == nil {
				if !ok {
					invalid = fmt.Errorf("missing or non-numeric chip temperature at board row %d, chip row %d", boardIndex+1, chipIndex+1)
				} else {
					invalid = fmt.Errorf("invalid chip temperature %.1f C at board row %d, chip row %d", temp, boardIndex+1, chipIndex+1)
				}
			}
		}
	}
	if c.cfg.AutomaticTuning && chipCount != 16 && invalid == nil {
		invalid = errors.New("automatic performance requires all 16 chip temperature readings")
	}
	if invalid != nil {
		return peak, invalid
	}
	if peak == 0 {
		return 0, errors.New("empty chip telemetry")
	}
	return peak, nil
}
