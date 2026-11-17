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

func appendStockLog(t *testing.T, path, text string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err = f.WriteString(text); err != nil {
		t.Fatal(err)
	}
}
func TestStockRecoveryReplaysFreshNativeRestart(t *testing.T) {
	now := time.Now()
	path := filepath.Join(t.TempDir(), "minerlog")
	appendStockLog(t, path, "old log\n")
	var s stockRecovery
	s.observe(now, path)
	appendStockLog(t, path, " [date] C0: Read Nonce Faild 10 Times, Reinit Device!!!\n [date] C0: gskb_reinit\n [date] C0: gskb_device_init...\n")
	s.observe(now.Add(time.Second), path)
	for i, clock := range []int{140, 350, 600, 740} {
		at := now.Add(time.Duration(1+i*3) * time.Second)
		wait, complete, err := s.check(at, defaultTuning, Tuning{430, clock}, true)
		if err != nil || wait != (clock != 740) || complete != (clock == 740) {
			t.Fatalf("clock %d: %t %t %v", clock, wait, complete, err)
		}
	}
	if !s.until.IsZero() {
		t.Fatal("recovery did not complete")
	}
	wait, _, _ := s.check(now.Add(12*time.Second), defaultTuning, Tuning{430, 140}, true)
	if wait {
		t.Fatal("old event authorized another recovery")
	}
}
func TestStockRecoveryRequiresFreshEvidenceAndSafeEnvelope(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		name     string
		event    time.Time
		reported Tuning
		verified bool
	}{
		{"no evidence", time.Time{}, Tuning{430, 140}, true},
		{"old evidence", now.Add(-7 * time.Second), Tuning{430, 140}, true},
		{"future evidence", now.Add(time.Second), Tuning{430, 140}, true},
		{"overvoltage", now, Tuning{440, 140}, true},
		{"undervoltage", now, Tuning{420, 140}, true},
		{"zero clock", now, Tuning{430, 0}, true},
		{"below ramp", now, Tuning{430, 40}, true},
		{"overclock", now, Tuning{430, 750}, true},
		{"unverified", now, Tuning{430, 140}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := stockRecovery{event: tc.event}
			wait, complete, _ := s.check(now, defaultTuning, tc.reported, tc.verified)
			if wait || complete || !s.until.IsZero() {
				t.Fatal("unauthorized recovery allowance")
			}
		})
	}
}
func TestStockRecoveryCannotExtendDeadlineOrConcealBadReadback(t *testing.T) {
	now := time.Now()
	for _, clock := range []int{49, 750} {
		s := stockRecovery{until: now.Add(stockRecoveryLimit)}
		if _, _, err := s.check(now, defaultTuning, Tuning{430, clock}, true); err == nil {
			t.Fatal("unsafe clock bypassed")
		}
	}
	s := stockRecovery{until: now.Add(stockRecoveryLimit)}
	if _, _, err := s.check(now, defaultTuning, Tuning{440, 140}, true); err == nil {
		t.Fatal("voltage bypassed")
	}
	s = stockRecovery{event: now}
	s.check(now, defaultTuning, Tuning{430, 140}, true)
	deadline := s.until
	for i := 1; i < 15; i++ {
		s.event = now.Add(time.Duration(i) * time.Second)
		wait, _, err := s.check(now.Add(time.Duration(i)*time.Second), defaultTuning, Tuning{430, 140}, true)
		if !wait || err != nil || !s.until.Equal(deadline) {
			t.Fatal("deadline moved")
		}
	}
	if _, _, err := s.check(deadline, defaultTuning, defaultTuning, true); err == nil {
		t.Fatal("late recovery accepted")
	}
	s = stockRecovery{until: deadline}
	if wait, _, err := s.check(now, defaultTuning, Tuning{}, false); !wait || err != nil {
		t.Fatal("unknown readback must retain recovery state")
	}
	if _, _, err := s.check(deadline, defaultTuning, Tuning{}, false); err == nil {
		t.Fatal("missing readback extended deadline")
	}
}
func TestStockRecoveryRejectsRepeatedRestartsAndUsesExpectedClock(t *testing.T) {
	now := time.Now()
	s := stockRecovery{event: now, lastAttempt: now.Add(-30 * time.Second)}
	if _, _, err := s.check(now, defaultTuning, Tuning{430, 140}, true); err == nil {
		t.Fatal("repeated restart accepted")
	}
	s.lastAttempt = now.Add(-time.Minute)
	wait, _, err := s.check(now, Tuning{430, 500}, Tuning{430, 140}, true)
	if !wait || err != nil {
		t.Fatal("qualified lower point was not supported")
	}
	if _, _, err := s.check(now.Add(time.Second), Tuning{430, 500}, Tuning{430, 510}, true); err == nil {
		t.Fatal("recovery exceeded lower expected point")
	}
}
func TestStockRecoveryLogCannotReuseHistoricalOrUntrustedMarkers(t *testing.T) {
	now := time.Now()
	path := filepath.Join(t.TempDir(), "minerlog")
	marker := " [date] C0: gskb_reinit\n"
	appendStockLog(t, path, marker)
	var s stockRecovery
	s.observe(now, path)
	if !s.event.IsZero() {
		t.Fatal("old log authorized recovery")
	}
	appendStockLog(t, path, " [date] C0: gskb_re")
	s.observe(now, path)
	if !s.event.IsZero() {
		t.Fatal("partial marker accepted")
	}
	appendStockLog(t, path, "init\n")
	s.observe(now, path)
	if s.event != now {
		t.Fatal("split fresh marker lost")
	}
	s.event = time.Time{}
	appendStockLog(t, path, " [date] C1: gskb_reinit\n")
	s.observe(now, path)
	if !s.event.IsZero() {
		t.Fatal("wrong board accepted")
	}
	os.Rename(path, path+".old")
	appendStockLog(t, path, marker)
	s.observe(now, path)
	if !s.event.IsZero() {
		t.Fatal("rotation replayed history")
	}
	os.WriteFile(path, []byte("x"), 0600)
	s.observe(now, path)
	if !s.event.IsZero() {
		t.Fatal("truncation replayed history")
	}
	appendStockLog(t, path, strings.Repeat("x", 17000)+marker)
	s.observe(now, path)
	if !s.event.IsZero() {
		t.Fatal("oversized gap trusted")
	}
	os.Remove(path)
	s.observe(now, path)
	if !s.event.IsZero() {
		t.Fatal("missing log retained authority")
	}
}
func TestStockRecoveryStatusAndSession(t *testing.T) {
	now := time.Now()
	c := Controller{lastGood: now, device: map[string]any{"hashrate": 700000.0}, stockRecovery: stockRecovery{until: now.Add(time.Second)}}
	c.updateMiningSession(now)
	if c.miningSession.status(now).Active || c.startupPhase() != "stock-recovery" {
		t.Fatal("restarting board counted as active")
	}
	c.beginStockStartup(now)
	if !c.stockRecovery.until.IsZero() {
		t.Fatal("explicit startup retained old recovery state")
	}
}

func TestStockRecoveryCannotCommitInterruptedAutomaticQualification(t *testing.T) {
	dir := t.TempDir()
	p := Pending{defaultTuning, Tuning{430, 750}}
	if err := atomicJSON(filepath.Join(dir, "pending.json"), p); err != nil {
		t.Fatal(err)
	}
	writes := 0
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/user/login" {
			w.Write([]byte(`{"JWT Token":"owner"}`))
			return
		}
		if r.Method == "PUT" {
			writes++
			var v map[string]any
			if json.NewDecoder(r.Body).Decode(&v) != nil || v["manualPowerplan"] != defaultTuning.Powerplan() {
				t.Error("wrong recovery operating point")
			}
		}
		w.Write([]byte(`{"manualPowerplan":"740 MHz 0.430 V 100 RPM 100 RPM"}`))
	}))
	defer backend.Close()
	c := Controller{dir: dir, backend: backend.URL, client: backend.Client(), firmware: "2.2.1", algorithm: 1, approved: defaultTuning, pending: &p, cfg: Config{AdminPassword: "unchanged", AutomaticTuning: true, AutomaticLimits: &performanceLimits{430, 850}}, started: time.Now().Add(-time.Hour), samples: 100}
	if !c.finishStockRecovery() || c.pending != nil || c.halted || writes != 1 || c.approved != defaultTuning || c.cfg.AdminPassword != "unchanged" || c.cfg.performanceLimits().MaximumClock != 740 {
		t.Fatal("interrupted qualification was not restored safely")
	}
	if _, err := os.Stat(filepath.Join(dir, "pending.json")); !os.IsNotExist(err) {
		t.Fatal("qualification journal remained")
	}
}
func TestStockRecoveryPreservesEstablishedTuningAndClearsObservations(t *testing.T) {
	now := time.Now()
	c := Controller{approved: Tuning{430, 755}, cfg: Config{AutomaticTuning: true, AutomaticLimits: &performanceLimits{430, 755}}, automatic: automaticPerformance{since: now}, stockRecovery: stockRecovery{lastAttempt: now}}
	if c.finishStockRecovery() || c.approved != (Tuning{430, 755}) || c.cfg.performanceLimits().MaximumClock != 755 || !c.automatic.since.IsZero() || c.cooling.percent != 100 || c.stockRecovery.lastAttempt != now {
		t.Fatal("completed recovery altered established tuning or retained observations")
	}
}
