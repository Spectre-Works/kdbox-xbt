package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHTTPWriteProtection(t *testing.T) {
	c := &Controller{sessions: map[string]Session{}, lastGood: time.Now()}
	for _, v := range []struct {
		origin, ctype string
		want          int
	}{{"http://evil.example", "application/json", 403}, {"", "text/plain", 415}, {"", "application/json", 401}} {
		r := httptest.NewRequest("POST", "http://miner/api/trial", strings.NewReader(`{"millivolts":430,"clock":745}`))
		r.Header.Set("Origin", v.origin)
		r.Header.Set("Content-Type", v.ctype)
		w := httptest.NewRecorder()
		c.handler(w, r)
		if w.Code != v.want {
			t.Fatalf("got %d want %d", w.Code, v.want)
		}
	}
}

func TestSessionCookieLoginAndLogout(t *testing.T) {
	native := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"JWT Token":"owner"}`))
	}))
	defer native.Close()
	c := &Controller{backend: native.URL, client: native.Client(), sessions: map[string]Session{}}
	login := httptest.NewRequest("POST", "http://miner/api/login", strings.NewReader(`{"password":"owner"}`))
	login.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c.handler(w, login)
	cookies := w.Result().Cookies()
	if w.Code != http.StatusOK || len(cookies) != 1 || cookies[0].Name != "kdbox-xbt" || !cookies[0].HttpOnly {
		t.Fatalf("unexpected login response: %d %v", w.Code, cookies)
	}
	cookie := cookies[0]
	if _, ok := c.sessions[cookie.Value]; !ok {
		t.Fatal("issued cookie does not identify a live session")
	}
	logout := httptest.NewRequest("POST", "http://miner/api/logout", strings.NewReader(`{}`))
	logout.Header.Set("Content-Type", "application/json")
	logout.AddCookie(cookie)
	w = httptest.NewRecorder()
	c.handler(w, logout)
	cleared := w.Result().Cookies()
	if w.Code != http.StatusOK || len(c.sessions) != 0 || len(cleared) != 1 || cleared[0].Name != cookie.Name || cleared[0].MaxAge >= 0 {
		t.Fatalf("logout did not invalidate the session: %d %v", w.Code, cleared)
	}
}
func TestAutomaticJournalPrecedesApplyAndRollback(t *testing.T) {
	dir := t.TempDir()
	puts := 0
	native := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user/login":
			w.Write([]byte(`{"JWT Token":"test"}`))
		case "/mcb/setting":
			if r.Method == "PUT" {
				puts++
				if _, e := os.Stat(filepath.Join(dir, "pending.json")); e != nil {
					t.Error("settings applied without durable journal")
				}
			}
			w.Write([]byte(`{"tempcontrol":true}`))
		default:
			t.Error(r.URL.Path)
		}
	}))
	defer native.Close()
	c := &Controller{dir: dir, backend: native.URL, client: native.Client(), approved: Tuning{430, 740}, cfg: Config{AutomaticLimits: &performanceLimits{430, 850}}, sessions: map[string]Session{"test": {time.Now().Add(time.Minute)}}, lastGood: time.Now(), device: map[string]any{"nonces": float64(100)}}
	if err := c.startPerformanceChange(Tuning{430, 750}, true, "Automatic clock adjustment"); err != nil {
		t.Fatal(err)
	}
	if c.pending == nil || puts != 1 {
		t.Fatal("automatic change missing")
	}
	if e := c.rollback("test"); e != nil {
		t.Fatal(e)
	}
	if puts != 2 || c.pending != nil || c.approved.Clock != 740 {
		t.Fatal("rollback incorrect")
	}
	b, e := os.ReadFile(filepath.Join(dir, "approved.json"))
	var restored Tuning
	if e != nil || json.Unmarshal(b, &restored) != nil || restored.Clock != 740 {
		t.Fatal("rollback not durable")
	}
	if _, e = os.Stat(filepath.Join(dir, "pending.json")); !os.IsNotExist(e) {
		t.Fatal("journal not cleared")
	}
}
func TestTemperature(t *testing.T) {
	for _, v := range []string{"", "unavailable", "0 C"} {
		if _, ok := temperature(v); ok {
			t.Fatal(v)
		}
	}
	v, ok := temperature("76.0 °C / 54.0 °C")
	if !ok || v != 76 {
		t.Fatal(v)
	}
}

func TestBootRollbackRetriesWithoutLatchingFault(t *testing.T) {
	dir := t.TempDir()
	failed := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/user/login" {
			w.Write([]byte(`{"JWT Token":"test"}`))
			return
		}
		if r.Method == "PUT" && !failed {
			failed = true
			w.WriteHeader(503)
			return
		}
		w.Write([]byte(`{"manualPowerplan":"740 MHz 0.430 V 100 RPM 100 RPM"}`))
	}))
	defer server.Close()
	previous := Tuning{430, 740}
	pending := Pending{previous, Tuning{425, 725}}
	if e := atomicJSON(filepath.Join(dir, "pending.json"), pending); e != nil {
		t.Fatal(e)
	}
	c := &Controller{dir: dir, backend: server.URL, client: server.Client(), pending: &pending, recovering: true}
	if c.rollback("boot") == nil {
		t.Fatal("expected transient failure")
	}
	if c.halted || c.pending == nil {
		t.Fatal("transient startup failure latched a stop or discarded recovery")
	}
	if e := c.rollback("boot"); e != nil {
		t.Fatal(e)
	}
	if c.halted || c.pending != nil || c.approved != previous {
		t.Fatal("boot retry failed")
	}
}

// Confirmation must not race the periodic safety loop after the deadline.
func TestManualTuningCannotConfirmAutomaticSettings(t *testing.T) {
	dir := t.TempDir()
	old := Tuning{430, 600}
	c := &Controller{dir: dir, approved: old, pending: &Pending{old, Tuning{430, 625}},
		sessions: map[string]Session{"test": {time.Now().Add(time.Minute)}},
		lastGood: time.Now(), started: time.Now().Add(-4 * time.Minute), deadline: time.Now().Add(-time.Second),
		samples: 20, baselineAccepted: 1, device: map[string]any{"accepted": float64(2)}}
	req := httptest.NewRequest("POST", "http://miner/api/confirm", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "kdbox-xbt", Value: "test"})
	w := httptest.NewRecorder()
	c.handler(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("manual confirmation endpoint remains available: %d", w.Code)
	}
	if c.approved != old || c.pending == nil {
		t.Fatal("expired confirmation changed approved settings")
	}
	if _, err := os.Stat(filepath.Join(dir, "approved.json")); !os.IsNotExist(err) {
		t.Fatal("expired confirmation wrote settings")
	}
}

func TestChipTelemetryRefreshAndMissingSensors(t *testing.T) {
	body := `{"drawdata":[[{"temp":72},{"temp":79}]]}`
	logins := 0
	native := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/user/login" {
			logins++
			w.Write([]byte(`{"JWT Token":"fresh"}`))
			return
		}
		if r.Header.Get("Authorization") != "Bearer fresh" {
			w.WriteHeader(401)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"body": body})
	}))
	defer native.Close()
	c := &Controller{backend: native.URL, client: native.Client(), token: "expired"}
	peak, err := c.chipPeak()
	if err != nil || peak != 79 || logins != 1 {
		t.Fatalf("peak=%v err=%v logins=%d", peak, err, logins)
	}
	for _, bad := range []struct{ body, message string }{
		{`{"drawdata":[]}`, "empty chip telemetry"},
		{`{"drawdata":[[{}]]}`, "missing or non-numeric chip temperature at board row 1, chip row 1"},
		{`{"drawdata":[[{"temp":60}],[{"temp":64},{"temp":0}]]}`, "invalid chip temperature 0.0 C at board row 2, chip row 2"},
		{`not-json`, "invalid chip telemetry"},
	} {
		body = bad.body
		if _, err = c.chipPeak(); err == nil || err.Error() != bad.message {
			t.Fatalf("body=%s err=%v want=%q", bad.body, err, bad.message)
		}
	}
	body = `{"drawdata":[[{}, {"temp":92}]]}`
	if peak, err := c.chipPeak(); peak != 92 || err == nil {
		t.Fatalf("partial report lost the known hot chip: peak=%v err=%v", peak, err)
	}
}

func TestDiagnosticsDuringSafetyStop(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "halted.json")
	if _, err := saveStop(path, "Temperature safety stop", map[string]any{"temp": "47.0 °C / 36.4 °C", "private": "do not return"}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	native := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user/login":
			w.Write([]byte(`{"JWT Token":"fresh"}`))
		case "/dbg/monitorlog":
			w.Write([]byte(`{"body":"monitor entry"}`))
		default:
			t.Errorf("unexpected stock request %s", r.URL.Path)
			w.WriteHeader(500)
		}
	}))
	defer native.Close()
	c := &Controller{dir: dir, halted: true, backend: native.URL, client: native.Client(), sessions: map[string]Session{"test": {time.Now().Add(time.Minute)}}}
	for _, tc := range []struct {
		kind     string
		unlocked bool
		status   int
		body     string
	}{
		{"safety-stop", false, 200, "47.0 °C / 36.4 °C"},
		{"monitorlog", false, 401, "Unlock"},
		{"icinfo", true, 503, "running mining engine"},
		{"monitorlog", true, 200, "monitor entry"},
	} {
		r := httptest.NewRequest("GET", "/api/diagnostics?kind="+tc.kind, nil)
		if tc.unlocked {
			r.AddCookie(&http.Cookie{Name: "kdbox-xbt", Value: "test"})
		}
		w := httptest.NewRecorder()
		c.handler(w, r)
		if w.Code != tc.status || !strings.Contains(w.Body.String(), tc.body) || strings.Contains(w.Body.String(), "do not return") {
			t.Errorf("%s, unlocked=%v: HTTP %d %s", tc.kind, tc.unlocked, w.Code, w.Body.String())
		}
	}
	after, err := os.ReadFile(path)
	if err != nil || !c.halted || string(before) != string(after) {
		t.Fatal("diagnostics disturbed the stop record", err)
	}
}

func TestDiagnosticsUsesNativeAuthentication(t *testing.T) {
	native := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/user/login" {
			w.Write([]byte(`{"JWT Token":"fresh"}`))
			return
		}
		if r.URL.Path != "/dbg/icinfo" || r.Header.Get("Authorization") != "Bearer fresh" {
			w.WriteHeader(401)
			return
		}
		w.Write([]byte(`{"body":"chip temperatures"}`))
	}))
	defer native.Close()
	c := &Controller{backend: native.URL, client: native.Client(), sessions: map[string]Session{"test": {time.Now().Add(time.Minute)}}}
	r := httptest.NewRequest("GET", "http://miner/api/diagnostics?kind=icinfo", nil)
	r.AddCookie(&http.Cookie{Name: "kdbox-xbt", Value: "test"})
	w := httptest.NewRecorder()
	c.handler(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "chip temperatures") {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestCoolingTargetIsAuthenticatedBoundedAndPersisted(t *testing.T) {
	dir := t.TempDir()
	c := Controller{dir: dir, cfg: Config{AdminPassword: "original", FallbackPassword: "recovery"},
		sessions: map[string]Session{"test": {time.Now().Add(time.Minute)}},
		halted:   true}
	if c.cfg.coolingTarget() != 75 || c.cfg.AutomaticTuning {
		t.Fatal("new installations must use a 75 C target with automatic performance disabled")
	}
	for _, tc := range []struct {
		body   string
		auth   bool
		status int
	}{
		{`{"target":80}`, false, 401},
		{`{"target":80}`, true, 200},
		{`{"target":69}`, true, 400},
		{`{"target":81}`, true, 400},
		{`{"target":70.5}`, true, 400},
		{`{}`, true, 400},
	} {
		r := httptest.NewRequest("POST", "/api/cooling", strings.NewReader(tc.body))
		r.Header.Set("Content-Type", "application/json")
		if tc.auth {
			r.AddCookie(&http.Cookie{Name: "kdbox-xbt", Value: "test"})
		}
		w := httptest.NewRecorder()
		c.handler(w, r)
		if w.Code != tc.status {
			t.Fatalf("body=%s status=%d want=%d", tc.body, w.Code, tc.status)
		}
	}
	var saved Config
	data, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil || json.Unmarshal(data, &saved) != nil || saved.coolingTarget() != 80 ||
		saved.AdminPassword != "original" || saved.FallbackPassword != "recovery" || !c.halted {
		t.Fatal("target save disturbed credentials, stop state, or persistence")
	}
}

func TestCoolingEditResetsOnlyTargetDependentObservations(t *testing.T) {
	now := time.Now()
	dir := t.TempDir()
	c := Controller{dir: dir, approved: Tuning{430, 755}, cfg: Config{AdminPassword: "original", ChipTarget: 70, AutomaticTuning: true, AutomaticLimits: &performanceLimits{430, 755}}, sessions: map[string]Session{"owner": {now.Add(time.Hour)}},
		cooling:   automaticCooling{percent: 67, target: 70, headroom: 5, headroomGoal: 5, raw: []coolingReading{{now, [3]float64{50, 60, 70}}}, boostUntil: now.Add(time.Minute)},
		automatic: automaticPerformance{since: now.Add(-4 * time.Minute), baseline: performanceReading{HW: 10, Nonces: 10000}, hotSince: now, coolSince: now}}
	approved := c.approved
	since := c.automatic.since
	baseline := c.automatic.baseline
	boost := c.cooling.boostUntil
	r := httptest.NewRequest("POST", "/api/cooling", strings.NewReader(`{"target":75}`))
	r.Header.Set("Content-Type", "application/json")
	r.AddCookie(&http.Cookie{Name: "kdbox-xbt", Value: "owner"})
	w := httptest.NewRecorder()
	c.handler(w, r)
	if w.Code != 200 || c.cooling.headroom != 0 || c.cooling.headroomGoal != 0 || len(c.cooling.raw) != 0 || c.cooling.percent != 67 || c.cooling.boostUntil != boost {
		t.Fatal("target state or boost changed incorrectly", w.Code, c.cooling)
	}
	if c.approved != approved || c.cfg.AdminPassword != "original" || !c.cfg.AutomaticTuning || c.cfg.performanceLimits().MaximumClock != 755 || c.automatic.since != since || c.automatic.baseline != baseline {
		t.Fatal("target edit discarded qualification or owner settings")
	}
	if !c.automatic.hotSince.IsZero() || !c.automatic.coolSince.IsZero() {
		t.Fatal("old target dwell retained")
	}
}
