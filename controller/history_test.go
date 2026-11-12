package main

import (
	"math"
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
