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

func TestAutomaticThermalBudgetAndBriefSpikes(t *testing.T) {
	now := time.Now()
	var a automaticPerformance
	r := performanceReading{Filtered: 70, Peak: 70, Nonces: 100}
	limits := performanceLimits{430, 740}
	a.next(now, 70, defaultTuning, limits, r)
	r.Peak = 77
	if d := a.next(now.Add(3*time.Second), 70, defaultTuning, limits, r); d.Setting != (Tuning{}) {
		t.Fatal(d)
	}
	r.Peak = 70
	if d := a.next(now.Add(6*time.Second), 70, defaultTuning, limits, r); d.Setting != (Tuning{}) {
		t.Fatal(d)
	}
	r.Filtered, r.Peak = 72, 72
	a.next(now.Add(10*time.Second), 70, defaultTuning, limits, r)
	if d := a.next(now.Add(39*time.Second), 70, defaultTuning, limits, r); d.Setting != (Tuning{}) {
		t.Fatal("heat dwell was bypassed", d)
	}
	d := a.next(now.Add(40*time.Second), 70, defaultTuning, limits, r)
	if d.Setting != (Tuning{430, 715}) || d.Verify || d.Stop {
		t.Fatal("thermal derating failed", d)
	}
	a = automaticPerformance{}
	a.next(now, 70, Tuning{425, 500}, limits, r)
	if !a.next(now.Add(31*time.Second), 70, Tuning{425, 500}, limits, r).Stop {
		t.Fatal("no operating point at minimum clock must stop")
	}
}

func TestAutomaticHeadroomVoltageAndHealthGates(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		current, want Tuning
		limits        performanceLimits
	}{
		{defaultTuning, Tuning{430, 750}, performanceLimits{430, 850}},
		{Tuning{430, 845}, Tuning{430, 850}, performanceLimits{430, 850}},
		{defaultTuning, Tuning{}, performanceLimits{430, 740}},
	} {
		var a automaticPerformance
		r := performanceReading{Filtered: 67, Peak: 68, Nonces: 100, Accepted: 1}
		a.next(now, 70, tc.current, tc.limits, r)
		r.Nonces = 4000
		if d := a.next(now.Add(301*time.Second), 70, tc.current, tc.limits, r); d.Setting != (Tuning{}) {
			t.Fatal("no new accepted share", d)
		}
		r.Accepted = 2
		d := a.next(now.Add(302*time.Second), 70, tc.current, tc.limits, r)
		if d.Setting != tc.want || (tc.want != (Tuning{}) && !d.Verify) {
			t.Fatal("wrong bounded proposal", d, tc.want)
		}
		r.HW = 20
		if d := a.next(now.Add(303*time.Second), 70, tc.current, tc.limits, r); d.Stop || !d.Quality || d.Setting.Clock >= tc.current.Clock {
			t.Fatal("hardware errors did not block optimization")
		}
	}
}

func TestPerformanceAPIHasNoManualTuning(t *testing.T) {
	c := Controller{dir: t.TempDir(), firmware: "2.2.1", algorithm: 1, halted: true, approved: defaultTuning,
		cfg: Config{AdminPassword: "original"}, sessions: map[string]Session{"owner": {time.Now().Add(time.Hour)}}}
	for _, tc := range []struct {
		path, body string
		auth       bool
		status     int
	}{
		{"performance", `{"automatic":true}`, false, 401},
		{"performance", `{"automatic":true}`, true, 200},
		{"performance", `{"automatic":false,"clock":800}`, true, 400},
		{"performance", `{}`, true, 400},
		{"trial", `{"clock":800,"millivolts":430}`, true, 404},
		{"confirm", `{}`, true, 404},
		{"revert", `{}`, true, 404},
		{"performance", `{"automatic":false}`, true, 200},
	} {
		r := httptest.NewRequest("POST", "/api/"+tc.path, strings.NewReader(tc.body))
		r.Header.Set("Content-Type", "application/json")
		if tc.auth {
			r.AddCookie(&http.Cookie{Name: "kdbox-xbt", Value: "owner"})
		}
		w := httptest.NewRecorder()
		c.handler(w, r)
		if w.Code != tc.status {
			t.Fatalf("%s: %d: %s", tc.path, w.Code, w.Body.String())
		}
	}
	var saved Config
	b, err := os.ReadFile(filepath.Join(c.dir, "config.json"))
	if err != nil || json.Unmarshal(b, &saved) != nil || saved.AdminPassword != "original" || saved.AutomaticTuning || !c.halted {
		t.Fatal("mode save disturbed credentials or stop state")
	}
}

func TestAutomaticCommitRequiresObservedWork(t *testing.T) {
	now := time.Now()
	dir := t.TempDir()
	p := Pending{defaultTuning, Tuning{430, 750}}
	if err := atomicJSON(filepath.Join(dir, "pending.json"), p); err != nil {
		t.Fatal(err)
	}
	c := Controller{dir: dir, cfg: Config{AutomaticTuning: true, AutomaticLimits: &performanceLimits{430, 850}}, approved: defaultTuning, pending: &p,
		started: now.Add(-6 * time.Minute), deadline: now.Add(time.Minute), baselineNonces: 100,
		device: map[string]any{"clock": float64(750), "voltage": .43, "nonces": float64(5000), "hwerrors": float64(0), "accepted": float64(0), "peak_temp": float64(69)}}
	c.cooling.peak = 69
	c.performanceSafety(now, true)
	if c.pending == nil {
		t.Fatal("accepted-share gate was skipped")
	}
	c.device["accepted"] = float64(1)
	c.performanceSafety(now.Add(time.Second), true)
	if c.pending != nil || c.approved != p.Candidate {
		t.Fatal("verified setting was not retained")
	}
	if _, err := os.Stat(filepath.Join(dir, "pending.json")); !os.IsNotExist(err) {
		t.Fatal("recovery journal was not cleared")
	}
}

func TestAutomaticRejectionRestoresPointAndPreservesFailureLimit(t *testing.T) {
	dir := t.TempDir()
	puts := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/user/login" {
			w.Write([]byte(`{"JWT Token":"owner"}`))
			return
		}
		if r.Method == "PUT" {
			puts++
			if _, err := os.Stat(filepath.Join(dir, "pending.json")); err != nil {
				t.Error("journal missing during restore")
			}
		}
		w.Write([]byte(`{"manualPowerplan":"740 MHz 0.430 V 100 RPM 100 RPM"}`))
	}))
	defer s.Close()
	p := Pending{defaultTuning, Tuning{430, 750}}
	atomicJSON(filepath.Join(dir, "pending.json"), p)
	now := time.Now()
	c := Controller{dir: dir, backend: s.URL, client: s.Client(), approved: defaultTuning, pending: &p,
		cfg: Config{AdminPassword: "original", AutomaticTuning: true, AutomaticLimits: &performanceLimits{430, 850}}, started: now.Add(-time.Minute), deadline: now.Add(time.Minute), baselineNonces: 100,
		device: map[string]any{"clock": float64(750), "voltage": .43, "nonces": float64(1100), "hwerrors": float64(10), "accepted": float64(1), "peak_temp": float64(69)}}
	c.performanceSafety(now, true)
	if c.halted || puts != 1 || c.pending != nil || c.approved != defaultTuning || c.cfg.performanceLimits().MaximumClock != 740 || c.cfg.AdminPassword != "original" {
		t.Fatal("automatic rejection did not recover safely", puts, c.approved, c.cfg.performanceLimits())
	}
	var cfg Config
	b, _ := os.ReadFile(filepath.Join(dir, "config.json"))
	json.Unmarshal(b, &cfg)
	if cfg.performanceLimits().MaximumClock != 740 {
		t.Fatal("rejected frequency would be retried after restart")
	}
}

func TestDefaultRestoreNeedsNoCustomerConfirmation(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/user/login" {
			w.Write([]byte(`{"JWT Token":"owner"}`))
			return
		}
		w.Write([]byte(`{}`))
	}))
	defer s.Close()
	c := Controller{dir: t.TempDir(), backend: s.URL, client: s.Client(), cfg: Config{AutomaticTuning: true, AutomaticLimits: &performanceLimits{430, 850}}, approved: Tuning{425, 500}}
	if err := c.setAutomaticPerformance(false); err != nil {
		t.Fatal(err)
	}
	if c.cfg.AutomaticTuning || c.pending != nil || c.approved != defaultTuning {
		t.Fatal("defaults were not restored automatically")
	}
}

func TestAutomaticCapsAndCompleteChipTelemetry(t *testing.T) {
	for _, point := range []Tuning{{430, 855}, {435, 740}, {425, 740}, {430, 495}} {
		if point.automaticBounds() == nil {
			t.Fatal("unqualified automatic point accepted", point)
		}
	}
	for _, limits := range []performanceLimits{{420, 740}, {435, 740}, {430, 855}, {430, 499}} {
		if (Config{AutomaticLimits: &limits}).validatePerformance() == nil {
			t.Fatal("invalid saved limits accepted", limits)
		}
	}
	c := Controller{firmware: "2.2.1", algorithm: 0}
	if c.setAutomaticPerformance(true) == nil {
		t.Fatal("automatic tuning enabled for an unqualified algorithm")
	}
	c.firmware, c.algorithm = "2.2.3", 1
	if c.setAutomaticPerformance(true) == nil {
		t.Fatal("automatic tuning enabled on an unqualified base")
	}
	chips := make([]map[string]any, 16)
	for i := range chips {
		chips[i] = map[string]any{"temp": 70}
	}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := json.Marshal(map[string]any{"drawdata": []any{chips}})
		json.NewEncoder(w).Encode(map[string]string{"body": string(b)})
	}))
	defer s.Close()
	c = Controller{backend: s.URL, client: s.Client(), cfg: Config{AutomaticTuning: true, AutomaticLimits: &performanceLimits{430, 850}}}
	if peak, err := c.chipPeak(); err != nil || peak != 70 {
		t.Fatal(peak, err)
	}
	chips = chips[:15]
	if _, err := c.chipPeak(); err == nil {
		t.Fatal("optimizer accepted an incomplete chip report")
	}
}

func TestLiveClockChangeIsJournaledAndKeepsVoltageAndObservationWindow(t *testing.T) {
	now := time.Now()
	dir := t.TempDir()
	p := Pending{defaultTuning, Tuning{430, 750}}
	if err := atomicJSON(filepath.Join(dir, "pending.json"), p); err != nil {
		t.Fatal(err)
	}
	c := Controller{dir: dir, cfg: Config{AutomaticTuning: true, AutomaticLimits: &performanceLimits{430, 850}}, approved: defaultTuning, pending: &p,
		started: now.Add(-301 * time.Second), deadline: now.Add(time.Minute), baselineNonces: 100, baselineAccepted: 1,
		device: map[string]any{"clock": float64(750), "voltage": .43, "hwerrors": float64(0), "nonces": float64(5000), "accepted": float64(3), "peak_temp": float64(67)}}
	c.cooling.peak = 67
	c.automatic = automaticPerformance{since: c.started, coolSince: now.Add(-3 * time.Minute), baseline: performanceReading{Nonces: 100, Accepted: 1}}
	c.reload = func() error { t.Fatal("live clock update restarted mining"); return nil }
	called := 0
	c.setClock = func(clock int) error {
		called++
		if clock != 760 {
			t.Fatal("wrong next clock", clock)
		}
		var journal Pending
		b, err := os.ReadFile(filepath.Join(dir, "pending.json"))
		if err != nil || json.Unmarshal(b, &journal) != nil || journal.Previous != (Tuning{430, 750}) || journal.Candidate != (Tuning{430, 760}) {
			t.Fatal("missing clock recovery record")
		}
		return nil
	}
	c.performanceSafety(now, true)
	if c.pending != nil || c.approved != (Tuning{430, 750}) {
		t.Fatal("qualification failed")
	}
	c.performanceSafety(now.Add(time.Second), true)
	if called != 1 || c.pending == nil || c.pending.Candidate.Millivolts != 430 || c.started.IsZero() {
		t.Fatal("clock-only progression failed")
	}
}

func TestSingleHardwareErrorIsNormalAndMissingCountersAreInvalid(t *testing.T) {
	now := time.Now()
	var a automaticPerformance
	r := performanceReading{Filtered: 60, Peak: 60, Nonces: 100, Accepted: 1}
	a.next(now, 80, defaultTuning, performanceLimits{430, 850}, r)
	r.HW, r.Nonces = 1, 101
	if d := a.next(now.Add(time.Second), 80, defaultTuning, performanceLimits{430, 850}, r); d.Stop || d.Setting != (Tuning{}) {
		t.Fatal("a single invalid nonce changed mining")
	}
	if _, _, _, ok := performanceCounters(map[string]any{"nonces": float64(100), "accepted": float64(1)}); ok {
		t.Fatal("missing HW counter was treated as zero")
	}
}

func TestFactoryClockLimitCannotChangeVoltageCredentialsOrCustomerControls(t *testing.T) {
	dir := t.TempDir()
	c := Controller{dir: dir, cfg: Config{AdminPassword: "original", ChipTarget: 80}, approved: defaultTuning}
	path := filepath.Join(dir, "clock-limit-request.json")
	if err := os.WriteFile(path, []byte(`{"maximum_clock":850}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := c.reloadFactoryClockLimit(); err != nil {
		t.Fatal(err)
	}
	if c.cfg.AdminPassword != "original" || c.cfg.ChipTarget != 80 || c.cfg.AutomaticTuning || c.cfg.performanceLimits() != (performanceLimits{430, 850}) || c.approved != defaultTuning {
		t.Fatal("factory limit changed unrelated settings")
	}
	for _, clock := range []int{0, 499, 851, 900} {
		b, _ := json.Marshal(map[string]int{"maximum_clock": clock})
		os.WriteFile(path, b, 0600)
		if c.reloadFactoryClockLimit() == nil {
			t.Fatal("unsafe factory bound accepted", clock)
		}
	}
}

func TestStartupAndRecoveryRespectTheUnitCeilingAtFixedVoltage(t *testing.T) {
	for _, tc := range []struct {
		cfg         Config
		saved, want Tuning
	}{
		{Config{}, Tuning{430, 850}, defaultTuning},
		{Config{AutomaticTuning: true}, Tuning{430, 850}, defaultTuning},
		{Config{AutomaticTuning: true, AutomaticLimits: &performanceLimits{430, 700}}, Tuning{430, 850}, Tuning{430, 700}},
		{Config{AutomaticTuning: true, AutomaticLimits: &performanceLimits{430, 850}}, Tuning{425, 740}, defaultTuning},
	} {
		if got := initialPerformancePoint(tc.cfg, tc.saved); got != tc.want {
			t.Fatal(got, tc.want)
		}
	}
}

func TestDefaultInvalidNoncesNeverStopOrAdjustMining(t *testing.T) {
	now := time.Now()
	for _, hw := range []any{float64(1), float64(10000), nil, "unavailable"} {
		c := Controller{dir: t.TempDir(), approved: defaultTuning, cfg: Config{}, device: map[string]any{"clock": float64(740), "voltage": .43, "hwerrors": hw}}
		c.performanceSafety(now, true)
		c.performanceSafety(now.Add(time.Minute), true)
		if c.halted || c.pending != nil || c.approved != defaultTuning {
			t.Fatal("default mining reacted to invalid nonces", hw)
		}
	}
}
func TestAutomaticErrorBudgetWindowsAndMinimumNeverRequestErrorStop(t *testing.T) {
	for _, tc := range []struct {
		hw, nonces float64
		excessive  bool
	}{{1, 1, false}, {4, 100, false}, {5, 100, true}, {5, 10000, false}, {20, 100000, true}, {-1, 100, false}} {
		if got := excessiveHardwareErrors(100, 1000, 100+tc.hw, 1000+tc.nonces); got != tc.excessive {
			t.Fatal(tc, got)
		}
	}
	now := time.Now()
	var a automaticPerformance
	r := performanceReading{Filtered: 70, Peak: 70, Nonces: 100, Accepted: 1}
	point := Tuning{430, 500}
	limits := performanceLimits{430, 740}
	a.next(now, 80, point, limits, r)
	r.HW = 20
	r.Nonces = 200
	if d := a.next(now.Add(time.Second), 80, point, limits, r); d.Stop || d.Setting != (Tuning{}) {
		t.Fatal("minimum clock stopped for invalid nonces", d)
	}
	// Expired windows cannot aggregate unrelated small batches forever.
	a = automaticPerformance{}
	r.HW = 0
	r.Nonces = 100
	a.next(now, 80, defaultTuning, limits, r)
	r.HW = 4
	r.Nonces = 200
	a.next(now.Add(301*time.Second), 80, defaultTuning, limits, r)
	r.HW = 8
	r.Nonces = 300
	if d := a.next(now.Add(302*time.Second), 80, defaultTuning, limits, r); d.Quality || d.Stop {
		t.Fatal("errors accumulated across windows", d)
	}
}

func TestEstablishedAutomaticErrorBackoffLearnsLowerClockWithoutStopping(t *testing.T) {
	now := time.Now()
	c := Controller{dir: t.TempDir(), approved: Tuning{430, 750}, cfg: Config{AdminPassword: "original", AutomaticTuning: true, AutomaticLimits: &performanceLimits{430, 850}},
		device: map[string]any{"clock": float64(750), "voltage": .43, "hwerrors": float64(100), "nonces": float64(10000), "accepted": float64(10), "peak_temp": float64(70)}}
	c.cooling.peak = 70
	calls := 0
	c.setClock = func(clock int) error {
		calls++
		if clock != 725 {
			t.Fatal(clock)
		}
		return nil
	}
	c.performanceSafety(now, true)
	c.device["hwerrors"], c.device["nonces"] = float64(110), float64(11000)
	c.performanceSafety(now.Add(time.Second), true)
	if c.halted || c.pending != nil || calls != 1 || c.approved != (Tuning{430, 725}) || c.cfg.performanceLimits().MaximumClock != 725 || c.cfg.AdminPassword != "original" {
		t.Fatal("error budget did not recover", c.approved, c.cfg.performanceLimits())
	}
	c.device["clock"] = float64(725)
	c.performanceSafety(now.Add(2*time.Second), true)
	if calls != 1 || c.halted {
		t.Fatal("old cumulative errors retriggered backoff")
	}
}
func TestIsolatedErrorDoesNotRejectCandidate(t *testing.T) {
	now := time.Now()
	dir := t.TempDir()
	p := Pending{defaultTuning, Tuning{430, 750}}
	atomicJSON(filepath.Join(dir, "pending.json"), p)
	c := Controller{dir: dir, approved: defaultTuning, pending: &p, cfg: Config{AutomaticTuning: true, AutomaticLimits: &performanceLimits{430, 850}}, started: now.Add(-6 * time.Minute), deadline: now.Add(time.Minute), baselineNonces: 100,
		device: map[string]any{"clock": float64(750), "voltage": .43, "hwerrors": float64(1), "nonces": float64(5100), "accepted": float64(1), "peak_temp": float64(70)}}
	c.cooling.peak = 70
	c.performanceSafety(now, true)
	if c.halted || c.pending != nil || c.approved != p.Candidate {
		t.Fatal("isolated error rejected otherwise stable accepted work")
	}
}
