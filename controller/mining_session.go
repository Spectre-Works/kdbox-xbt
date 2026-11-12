package main

import (
	"math"
	"time"
)

// Tracks observed Mining active transitions independently of browser lifetime.
// A dropped browser connection does not reset an otherwise healthy miner.
type miningSession struct {
	since, lastReading time.Time
}
type miningSessionStatus struct {
	Active         bool   `json:"active"`
	StartedAt      string `json:"started_at,omitempty"`
	ElapsedSeconds int64  `json:"elapsed_seconds"`
}

func (s *miningSession) observe(now, reading time.Time, active bool) {
	if !active || reading.IsZero() || now.Sub(reading) > 15*time.Second {
		*s = miningSession{}
		return
	}
	// A stale interval must count as leaving Mining active even when no browser
	// happened to request status during the gap.
	if s.since.IsZero() || (!s.lastReading.IsZero() && reading.Sub(s.lastReading) > 15*time.Second) {
		s.since = reading
	}
	s.lastReading = reading
}
func (s miningSession) status(now time.Time) miningSessionStatus {
	if s.since.IsZero() {
		return miningSessionStatus{}
	}
	return miningSessionStatus{Active: true, StartedAt: s.since.UTC().Format(time.RFC3339), ElapsedSeconds: max(0, int64(now.Sub(s.since)/time.Second))}
}

// Caller holds c.mu. Valid telemetry is published by the single safety worker.
func (c *Controller) updateMiningSession(now time.Time) {
	rate := number(c.device["hashrate"])
	c.miningSession.observe(now, c.lastGood, !c.halted && c.startup.phase() == "" && rate > 0 && !math.IsNaN(rate) && !math.IsInf(rate, 0))
}
