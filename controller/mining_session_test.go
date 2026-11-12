package main

import (
	"encoding/json"
	"math"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMiningSessionTracksActiveTransitionsAndFreshnessWithoutBrowsers(t *testing.T) {
	start := time.Unix(2000000000, 0)
	var session miningSession
	session.observe(start, start, false)
	if session.status(start).Active {
		t.Fatal("idle miner started a session")
	}
	session.observe(start, start, true)
	// Observations are made by the safety worker, without any status requests.
	for i := 1; i <= 1200; i++ {
		at := start.Add(time.Duration(i) * 3 * time.Second)
		session.observe(at, at, true)
	}
	s := session.status(start.Add(time.Hour))
	if !s.Active || s.ElapsedSeconds != 3600 || s.StartedAt != start.UTC().Format(time.RFC3339) {
		t.Fatal(s)
	}
	// A short telemetry delay does not leave the existing active state.
	session.observe(start.Add(time.Hour+12*time.Second), start.Add(time.Hour), true)
	if session.status(start.Add(time.Hour+12*time.Second)).ElapsedSeconds != 3612 {
		t.Fatal("brief delay reset uptime")
	}
	// No status request need occur while stale: a newly valid sample after the
	// freshness gap must still start a new interval.
	resumed := start.Add(time.Hour + 20*time.Second)
	session.observe(resumed, resumed, true)
	if session.status(resumed).ElapsedSeconds != 0 || session.since != resumed {
		t.Fatal("unobserved stale gap was counted as mining")
	}
	session.observe(resumed.Add(3*time.Second), resumed.Add(3*time.Second), false)
	if session.status(resumed).Active {
		t.Fatal("zero hashing/paused state retained interval")
	}
	session.observe(resumed.Add(time.Minute), resumed.Add(time.Minute), true)
	if session.status(resumed.Add(time.Minute)).ElapsedSeconds != 0 {
		t.Fatal("return to active did not reset")
	}
	session.observe(resumed.Add(2*time.Minute), resumed.Add(time.Minute), true)
	if session.status(resumed.Add(2 * time.Minute)).Active {
		t.Fatal("stale telemetry retained active interval")
	}
}
func TestSessionStatusSurvivesRepeatedReadsAndExcludesStartupStoppedAndInvalidRates(t *testing.T) {
	now := time.Now()
	c := Controller{device: map[string]any{"hashrate": float64(700000)}, lastGood: now}
	c.updateMiningSession(now)
	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		c.handler(w, httptest.NewRequest("GET", "/api/status", nil))
		var status struct {
			Mining miningSessionStatus `json:"mining_session"`
		}
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &status) != nil || !status.Mining.Active || status.Mining.StartedAt != now.UTC().Format(time.RFC3339) {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	for _, rate := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		c.device["hashrate"] = rate
		c.updateMiningSession(now)
		if c.miningSession.status(now).Active {
			t.Fatal("invalid rate counted", rate)
		}
	}
	c.device["hashrate"] = float64(700000)
	c.halted = true
	c.updateMiningSession(now)
	if c.miningSession.status(now).Active {
		t.Fatal("stopped miner counted")
	}
	c.halted = false
	c.firmware = "2.2.1"
	c.algorithm = 1
	c.beginStockStartup(now)
	if c.miningSession.status(now).Active {
		t.Fatal("calibration retained previous session")
	}
	c.device = map[string]any{"hashrate": float64(700000)}
	c.lastGood = now
	c.updateMiningSession(now)
	if c.miningSession.status(now).Active {
		t.Fatal("startup/calibration counted as Mining active")
	}
	c.algorithm = 0
	c.beginStockStartup(now)
	if c.device != nil || !c.lastGood.IsZero() {
		t.Fatal("algorithm restart retained the previous session telemetry")
	}
}
