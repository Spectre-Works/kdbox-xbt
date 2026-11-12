package main

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

func allowedOrigin(origin, host string) bool {
	u, e := url.Parse(origin)
	h := strings.Split(host, ":")[0]
	return e == nil && u.Scheme == "http" && u.Hostname() == h && (u.Port() == "" || u.Port() == "80" || u.Port() == "8088") && u.Path == "" && u.User == nil && u.RawQuery == "" && u.Fragment == ""
}

type NetworkTrial struct {
	Previous json.RawMessage `json:"previous"`
	Deadline time.Time       `json:"deadline"`
}
type NetworkSettings struct {
	DHCP bool              `json:"useDHCP"`
	Auto bool              `json:"useAuto"`
	IP   map[string]string `json:"setIP"`
}

func (n NetworkSettings) validate() error {
	for k, v := range n.IP {
		if k != "IPv4addr" && k != "Subnetmask" && k != "Router" && k != "DNS1" && k != "DNS2" {
			return errors.New("unknown network field")
		}
		if v != "" && (net.ParseIP(v) == nil || net.ParseIP(v).To4() == nil) {
			return errors.New("enter valid IPv4 addresses")
		}
	}
	if !n.DHCP {
		for _, k := range []string{"IPv4addr", "Subnetmask", "Router"} {
			if n.IP[k] == "" {
				return errors.New("static IP, mask and gateway are required")
			}
		}
		mask := net.IPMask(net.ParseIP(n.IP["Subnetmask"]).To4())
		ones, bits := mask.Size()
		if bits != 32 || ones == 0 {
			return errors.New("invalid subnet mask")
		}
	}
	if !n.Auto && n.IP["DNS1"] == "" {
		return errors.New("DNS 1 is required when automatic DNS is off")
	}
	return nil
}
func (c *Controller) restoreNetwork() error {
	if c.networkPending == nil {
		return nil
	}
	token, e := c.login(c.cfg.AdminPassword)
	if e != nil {
		return e
	}
	c.token = token
	if _, e = c.native("PUT", "/mcb/ip", c.networkPending.Previous, true); e != nil {
		return e
	}
	if e = os.Remove(filepath.Join(c.dir, "network-pending.json")); e != nil {
		return e
	}
	c.networkPending = nil
	c.message = "Previous network configuration restored"
	return nil
}
func (c *Controller) settings(w http.ResponseWriter, r *http.Request) bool {
	path := strings.TrimPrefix(r.URL.Path, "/api/")
	reads := map[string]string{"network": "/mcb/ip", "wifi": "/mcb/wifisetting", "pools": "/mcb/pools", "system": "/mcb/status", "cloud": "/mcb/cmossetting", "rgb": "/mcb/rgbsetting", "device-settings": "/mcb/setting"}
	if endpoint, ok := reads[path]; ok && r.Method == "GET" {
		b, e := c.nativeContext(r.Context(), "GET", endpoint, nil, false)
		if e != nil {
			jsonReply(w, 502, map[string]string{"error": "Device settings unavailable"})
		} else {
			if path == "pools" {
				b = poolTelemetry(b)
			}
			jsonReply(w, 200, json.RawMessage(b))
		}
		return true
	}
	if path == "network-trial" && r.Method == "GET" {
		jsonReply(w, 200, c.networkPending)
		return true
	}
	if r.Method != "POST" {
		return false
	}
	var body json.RawMessage
	switch path {
	case "network", "network-confirm", "wifi", "pool-add", "pool-delete", "pools", "cloud", "rgb", "identify", "password", "reboot", "logout":
	default:
		return false
	}
	if e := decode(r, &body); e != nil {
		jsonReply(w, 400, map[string]string{"error": "Invalid JSON"})
		return true
	}
	if c.pending != nil && path != "logout" {
		jsonReply(w, 409, map[string]string{"error": "Automatic performance is adjusting; please wait"})
		return true
	}
	token, e := c.login(c.cfg.AdminPassword)
	if e != nil {
		jsonReply(w, 502, map[string]string{"error": "Controller authentication failed"})
		return true
	}
	c.token = token
	switch path {
	case "network":
		var n NetworkSettings
		e = json.Unmarshal(body, &n)
		if e == nil {
			e = n.validate()
		}
		if e == nil && c.networkPending != nil {
			e = errors.New("confirm or wait for the current network trial")
		}
		if e == nil {
			var previous []byte
			previous, e = c.native("GET", "/mcb/ip", nil, false)
			if e == nil {
				p := &NetworkTrial{previous, time.Now().Add(2 * time.Minute)}
				e = atomicJSON(filepath.Join(c.dir, "network-pending.json"), p)
				if e == nil {
					c.networkPending = p
					_, e = c.native("PUT", "/mcb/ip", n, true)
				}
			}
		}
	case "network-confirm":
		if c.networkPending != nil {
			e = os.Remove(filepath.Join(c.dir, "network-pending.json"))
			if e == nil {
				c.networkPending = nil
			}
		}
	case "wifi":
		var v map[string]json.RawMessage
		e = json.Unmarshal(body, &v)
		if len(v) != 1 {
			e = errors.New("one Wi-Fi operation at a time")
		}
		for k := range v {
			if k != "enable" && k != "connect" && k != "disconnect" && k != "forget" && k != "scan" {
				e = errors.New("unknown Wi-Fi operation")
			}
		}
		if e == nil {
			_, e = c.native("PUT", "/mcb/wifisetting", body, true)
		}
	case "pool-add", "pool-delete":
		var p struct {
			URL  string `json:"url"`
			User string `json:"user"`
			Pass string `json:"pass"`
		}
		e = json.Unmarshal(body, &p)
		if e == nil {
			var address string
			address, e = normalizePool(p.URL, p.User)
			if path == "pool-add" {
				p.URL = address
			}
		}
		if e == nil {
			current, err := c.native("GET", "/mcb/pools", nil, false)
			var entries []map[string]any
			if err != nil || json.Unmarshal(current, &entries) != nil {
				e = errors.New("could not read current pools")
			} else if path == "pool-delete" && len(entries) <= 1 {
				e = errors.New("keep at least one pool")
			} else if path == "pool-add" && len(entries) >= 10 {
				e = errors.New("maximum of 10 pools")
			}
		}
		if e == nil {
			if path == "pool-delete" {
				_, e = c.native("PUT", "/mcb/delpool", body, true)
			} else {
				_, e = c.native("PUT", "/mcb/newpool", p, true)
			}
		}
	case "pools":
		var pools []map[string]any
		e = json.Unmarshal(body, &pools)
		if len(pools) < 1 || len(pools) > 10 {
			e = errors.New("keep between 1 and 10 pools")
		}
		if e == nil {
			for i := range pools {
				address, _ := pools[i]["url"].(string)
				worker, _ := pools[i]["user"].(string)
				if _, e = normalizePool(address, worker); e != nil {
					break
				}
				pools[i]["pool-priority"] = i
			}
			if e == nil {
				_, e = c.native("PUT", "/mcb/pools", pools, true)
			}
		}
	case "cloud", "rgb":
		endpoint := "/mcb/cmossetting"
		if path == "rgb" {
			endpoint = "/mcb/rgbsetting"
		}
		_, e = c.native("PUT", endpoint, body, true)
	case "identify":
		var q struct {
			Enabled bool `json:"enabled"`
		}
		e = json.Unmarshal(body, &q)
		if e == nil {
			var b []byte
			b, e = c.native("GET", "/mcb/setting", nil, false)
			var v map[string]any
			if e == nil {
				e = json.Unmarshal(b, &v)
			}
			if e == nil {
				v["ledcontrol"] = q.Enabled
				v["tempcontrol"] = true
				_, e = c.native("PUT", "/mcb/setting", v, true)
			}
		}
	case "password":
		var q struct {
			Old     string `json:"old"`
			New     string `json:"new"`
			Confirm string `json:"confirm"`
		}
		e = json.Unmarshal(body, &q)
		if q.New == "" || q.New != q.Confirm || len(q.New) > 128 {
			e = errors.New("new passwords must match (1Ã¢â‚¬â€œ128 characters)")
		}
		if e == nil {
			_, e = c.login(q.Old)
		}
		if e == nil {
			old := c.cfg
			next := c.cfg
			next.AdminPassword, next.FallbackPassword = q.New, q.Old
			e = atomicJSON(filepath.Join(c.dir, "config.json"), next)
			if e == nil {
				query := url.Values{"oldpassword": {q.Old}, "newpassword": {q.New}, "cfmpassword": {q.Confirm}}
				_, e = c.native("PUT", "/user/updatepd?"+query.Encode(), nil, true)
				if e == nil {
					_, e = c.login(q.New)
				}
				if e == nil {
					c.cfg = next
					c.cfg.FallbackPassword = ""
					e = atomicJSON(filepath.Join(c.dir, "config.json"), c.cfg)
					c.sessions = map[string]Session{}
				} else {
					_ = atomicJSON(filepath.Join(c.dir, "config.json"), old)
				}
			}
		}
	case "reboot":
		var q struct {
			Confirm string `json:"confirm"`
		}
		_ = json.Unmarshal(body, &q)
		if q.Confirm != "REBOOT" {
			e = errors.New("type REBOOT to confirm")
		} else {
			_, e = c.native("PUT", "/mcb/restart", map[string]any{}, true)
		}
	case "logout":
		if cookie, e := r.Cookie("kdbox-xbt"); e == nil {
			delete(c.sessions, cookie.Value)
		}
		http.SetCookie(w, &http.Cookie{Name: "kdbox-xbt", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	}
	if e == nil && (path == "pool-add" || path == "pool-delete" || path == "pools") {
		c.engineGrace = time.Now().Add(90 * time.Second)
	}
	if e != nil {
		jsonReply(w, 400, map[string]string{"error": e.Error()})
	} else {
		jsonReply(w, 200, map[string]bool{"ok": true})
	}
	return true
}

var poolHostLabel = regexp.MustCompile(`(?i)^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

func poolAddress(address string) (string, error) {
	address = strings.TrimSpace(address)
	if address == "" || len(address) > 512 || strings.IndexFunc(address, unicode.IsSpace) >= 0 {
		return "", errors.New("enter a pool hostname and port without spaces")
	}
	scheme, target, explicit := strings.Cut(address, "://")
	if !explicit {
		scheme, target = "stratum+tcp", address
	}
	scheme = strings.ToLower(scheme)
	switch scheme {
	case "tcp", "ssl":
		scheme = "stratum+" + scheme
	case "stratum+tcp", "stratum+ssl":
	default:
		return "", errors.New("use a Stratum TCP or SSL pool address")
	}
	target = strings.TrimSuffix(target, "/")
	if strings.ContainsAny(target, "/@?#") {
		return "", errors.New("enter only the pool hostname and port")
	}
	host, port, err := net.SplitHostPort(target)
	if err != nil {
		return "", errors.New("include the pool port and put IPv6 addresses in brackets")
	}
	p, err := strconv.Atoi(port)
	if err != nil || strings.HasPrefix(port, "+") || p < 1 || p > 65535 {
		return "", errors.New("enter a pool port from 1 to 65535")
	}
	if net.ParseIP(host) == nil {
		if len(host) > 253 || strings.HasPrefix(target, "[") {
			return "", errors.New("enter a valid pool hostname or IP address")
		}
		for _, label := range strings.Split(strings.TrimSuffix(host, "."), ".") {
			if !poolHostLabel.MatchString(label) {
				return "", errors.New("enter a valid pool hostname or IP address")
			}
		}
	}
	return scheme + "://" + net.JoinHostPort(host, port), nil
}

func normalizePool(address, worker string) (string, error) {
	if strings.TrimSpace(worker) == "" || len(worker) > 256 || strings.ContainsAny(worker, "\r\n\x00") {
		return "", errors.New("enter a valid payout address or worker name")
	}
	return poolAddress(address)
}

func poolTelemetry(body []byte) []byte {
	var pools []map[string]any
	if json.Unmarshal(body, &pools) != nil {
		return body
	}
	live, err := minerRPC("pools")
	var rows []map[string]any
	if err == nil {
		_ = json.Unmarshal(live["POOLS"], &rows)
	}
	addPoolTelemetry(pools, rows)
	result, err := json.Marshal(pools)
	if err != nil {
		return body
	}
	return result
}

func addPoolTelemetry(pools, rows []map[string]any) {
	for _, p := range pools {
		p["mining"] = false
		p["telemetry_available"] = false
		address, _ := p["url"].(string)
		worker, _ := p["user"].(string)
		configured, err := poolAddress(address)
		if err != nil || worker == "" {
			continue
		}
		for _, row := range rows {
			address, _ := row["URL"].(string)
			liveWorker, _ := row["User"].(string)
			actual, err := poolAddress(address)
			if err == nil && strings.EqualFold(actual, configured) && liveWorker == worker {
				p["telemetry_available"] = true
				p["mining"] = row["Stratum Active"] == true
				p["accepted"] = row["Accepted"]
				p["connected"] = row["Status"] == "Alive"
				break
			}
		}
	}
}
