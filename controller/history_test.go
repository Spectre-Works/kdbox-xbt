package main

import (
	"math"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestHistoryPaddingGapsRetentionAndRestart(t *testing.T) {
	now := time.Unix(2000000040, 0).Truncate(time.Minute)
	h := &hashrateHistory{path: filepath.Join(t.TempDir(), "history.json")}
	if err := h.merge(now, []float64{0, 0, 700000, 0, 710000}); err != nil {
		t.Fatal(err)
	}
	v := h.snapshot(now)
	if len(v) != 3 || v[0].At != now.Add(-2*time.Minute).Unix() || v[1].Rate != 0 {
		t.Fatal("padding or a real zero-rate period was misrepresented", v)
	}
	if err := h.merge(now.Add(2*time.Minute), []float64{710000, math.NaN(), 720000}); err != nil {
		t.Fatal(err)
	}
	v = h.snapshot(now.Add(2 * time.Minute))
	if len(v) != 4 || v[3].At-v[2].At != 120 {
		t.Fatal("missing samples must remain a gap", v)
	}
	restored := &hashrateHistory{path: h.path}
	restored.load(now.Add(2 * time.Minute))
	if len(restored.snapshot(now.Add(2*time.Minute))) != len(v) {
		t.Fatal("controller restart lost retained history")
	}
	for i := 3; i < 1500; i++ {
		// Keep the retention test in memory instead of writing 1,500 files.
		restored.path = ""
		if err := restored.merge(now.Add(time.Duration(i)*time.Minute), []float64{700000}); err != nil {
			t.Fatal(err)
		}
	}
	v = restored.snapshot(now.Add(1499 * time.Minute))
	if len(v) != 1440 || v[len(v)-1].At-v[0].At != 1439*60 {
		t.Fatal("history did not retain exactly the last 24 hours", len(v))
	}
}

func TestHistoryStorageIsBoundedForNinetyDaysAndIrregularSnapshots(t *testing.T) {
	now := time.Unix(2000000000, 0)
	h := hashrateHistory{}
	for minute := 0; minute < 90*24*60; minute++ {
		at := now.Add(time.Duration(minute) * time.Minute)
		if err := h.merge(at, []float64{700000}); err != nil {
			t.Fatal(err)
		}
		if len(h.samples) > 1440 || cap(h.samples) > 3000 {
			t.Fatal("history storage grew", len(h.samples), cap(h.samples))
		}
	}
	// No allocation/copy on a read when nothing expired.
	before := &h.samples[0]
	h.prune(now.Add((90*24*60 - 1) * time.Minute))
	if before != &h.samples[0] {
		t.Fatal("prune copied unchanged history")
	}
	// A malformed prior in-memory source must still be capped by count.
	h.samples = make([]hashrateSample, 10000)
	for i := range h.samples {
		h.samples[i].At = now.Unix() + int64(i)
	}
	h.prune(now)
	if len(h.samples) > 1440 {
		t.Fatal("irregular timestamps bypassed count cap")
	}
}

func TestHistoryStopOverridesFrozenNativeRateAndSurvivesResume(t *testing.T) {
	now := time.Now().Truncate(time.Minute)
	requests := 0
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++; w.Write([]byte(`[0,680000,690000,999999]`)) }))
	defer backend.Close()
	c := Controller{backend: backend.URL, client: backend.Client(), lastGood: now, history: &hashrateHistory{path: filepath.Join(t.TempDir(), "history.json")}, device: map[string]any{"hashrate": 700000.0}}
	if err := c.collectHistory(now); err != nil {
		t.Fatal(err)
	}
	v := c.history.snapshot(now)
	if requests != 1 || len(v) != 3 || v[2].Rate != 700000 {
		t.Fatal("native current bucket replaced live rate", v, requests)
	}
	c.halted = true
	if err := c.collectHistory(now.Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if got := c.history.snapshot(now); got[len(got)-1].Rate != 0 {
		t.Fatal("stop kept the last live rate", got)
	}
	for i := 1; i <= 3; i++ {
		if err := c.collectHistory(now.Add(time.Duration(i) * time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	if requests != 1 {
		t.Fatal("stopped miner still read frozen native history")
	}
	restored := &hashrateHistory{path: c.history.path}
	restored.load(now.Add(3 * time.Minute))
	v = restored.snapshot(now.Add(3 * time.Minute))
	for _, p := range v {
		if p.At >= now.Unix() && p.Rate != 0 {
			t.Fatal("stopped interval was not retained", v)
		}
	}
	c.history = restored
	c.halted = false
	c.lastGood = now.Add(4 * time.Minute)
	c.device["hashrate"] = 710000.0
	if err := c.collectHistory(c.lastGood); err != nil {
		t.Fatal(err)
	}
	v = c.history.snapshot(c.lastGood)
	if v[len(v)-1].Rate != 710000 || requests != 1 {
		t.Fatal("resume replayed native backlog", v, requests)
	}
	for _, p := range v {
		if p.At >= now.Unix() && p.At < c.lastGood.Unix() && p.Rate != 0 {
			t.Fatal("resume overwrote the stopped interval")
		}
	}
}
func TestHistoryUnknownTelemetryLeavesGapsAndStartupRecordsZero(t *testing.T) {
	now := time.Now().Truncate(time.Minute)
	c := Controller{lastGood: now, device: map[string]any{"hashrate": 700000.0}, history: &hashrateHistory{}}
	c.history.record(now, 700000)
	c.collectHistory(now.Add(time.Minute)) // Last device snapshot is stale.
	if len(c.history.snapshot(now.Add(time.Minute))) != 1 {
		t.Fatal("unknown telemetry invented activity")
	}
	c.startup.calibrationUntil = now.Add(3 * time.Minute)
	c.collectHistory(now.Add(2 * time.Minute))
	c.startup = engineStartup{}
	c.stockRecovery.until = now.Add(4 * time.Minute)
	c.collectHistory(now.Add(3 * time.Minute))
	v := c.history.snapshot(now.Add(3 * time.Minute))
	if len(v) != 3 || v[1].At-v[0].At != 120 || v[1].Rate != 0 || v[2].Rate != 0 {
		t.Fatal("startup/recovery or missing telemetry was misrepresented", v)
	}
}
func TestHistoryStopDuringBootstrapCannotSeedPhantomHashing(t *testing.T) {
	now := time.Now()
	c := Controller{history: &hashrateHistory{}, lastGood: now, device: map[string]any{"hashrate": 700000.0}}
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.mu.Lock()
		c.halted = true
		c.mu.Unlock()
		w.Write([]byte(`[700000,700000,700000]`))
	}))
	defer backend.Close()
	c.backend = backend.URL
	c.client = backend.Client()
	if err := c.collectHistory(now); err != nil {
		t.Fatal(err)
	}
	v := c.history.snapshot(now)
	if len(v) != 1 || v[0].Rate != 0 {
		t.Fatal("bootstrap ignored a concurrent stop", v)
	}
}
func TestHistoryRepairsOnlyKnownStoppedIntervalAndRemainsBounded(t *testing.T) {
	now := time.Now().Truncate(time.Minute)
	h := hashrateHistory{path: filepath.Join(t.TempDir(), "history.json")}
	for i := -5; i <= 0; i++ {
		h.record(now.Add(time.Duration(i)*time.Minute), 700000)
	}
	since := now.Add(-3*time.Minute + 20*time.Second)
	if err := h.zeroSince(now, since); err != nil {
		t.Fatal(err)
	}
	v := h.snapshot(now)
	if len(v) != 6 {
		t.Fatal(v)
	}
	for _, p := range v {
		if p.At < since.Truncate(time.Minute).Unix() && p.Rate != 700000 {
			t.Fatal("rewrote pre-stop mining")
		}
		if p.At >= since.Truncate(time.Minute).Unix() && p.Rate != 0 {
			t.Fatal("frozen stopped history remained")
		}
	}
	h.zeroSince(now, now.Add(time.Minute))
	if len(h.snapshot(now)) != 6 {
		t.Fatal("future stop rewrote history")
	}
	h.zeroSince(now, time.Time{})
	if len(h.snapshot(now)) != 6 {
		t.Fatal("missing stop timestamp rewrote history")
	}
	h.zeroSince(now, now.Add(-72*time.Hour))
	v = h.snapshot(now)
	if len(v) != 1440 || v[0].Rate != 0 || v[len(v)-1].Rate != 0 {
		t.Fatal("repair exceeded retention bound", len(v))
	}
}
func TestHistoryRecordKeepsMinuteCadenceAndZeroTransitions(t *testing.T) {
	now := time.Now().Truncate(time.Minute)
	h := hashrateHistory{}
	h.record(now, 700000)
	h.record(now.Add(3*time.Second), 710000)
	if len(h.samples) != 1 || h.samples[0].Rate != 700000 {
		t.Fatal("normal polling rewrote the minute sample")
	}
	h.record(now.Add(6*time.Second), 0)
	if h.samples[0].Rate != 0 {
		t.Fatal("stop waited for the next minute")
	}
	h.record(now.Add(9*time.Second), 720000)
	if h.samples[0].Rate != 720000 {
		t.Fatal("resume retained zero")
	}
	h.record(now.Add(time.Minute), math.NaN())
	h.record(now.Add(time.Minute), math.Inf(1))
	h.record(now.Add(time.Minute), -1)
	if len(h.samples) != 1 {
		t.Fatal("invalid sample accepted")
	}
}
