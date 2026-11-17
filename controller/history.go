package main

import (
	"encoding/json"
	"io"
	"math"
	"os"
	"sync"
	"time"
)

type hashrateSample struct {
	At   int64   `json:"at"`
	Rate float64 `json:"rate"`
}

// Stock supplies 288 one-minute samples, with leading zero padding. Retain
// timestamped samples for 24 hours in RAM-backed /tmp, independently of browsers.
type hashrateHistory struct {
	mu      sync.Mutex
	path    string
	samples []hashrateSample
}

func (h *hashrateHistory) load(now time.Time) {
	f, err := os.Open(h.path)
	if err != nil {
		return
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 128*1024+1))
	if err != nil || len(data) > 128*1024 {
		return
	}
	var saved []hashrateSample
	if json.Unmarshal(data, &saved) != nil || len(saved) > 1440 {
		return
	}
	var last int64
	for _, s := range saved {
		if s.At <= last || s.At > now.Unix() || s.Rate < 0 || math.IsNaN(s.Rate) || math.IsInf(s.Rate, 0) {
			return
		}
		last = s.At
	}
	h.samples = saved
	h.prune(now)
}

func (h *hashrateHistory) prune(now time.Time) {
	cutoff := now.Unix() - 24*60*60
	i := 0
	for i < len(h.samples) && h.samples[i].At <= cutoff {
		i++
	}
	i = max(i, len(h.samples)-1440)
	if i > 0 {
		copy(h.samples, h.samples[i:])
		h.samples = h.samples[:len(h.samples)-i]
	}
}

func (h *hashrateHistory) merge(now time.Time, raw []float64) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.prune(now)
	if len(raw) > 288 {
		raw = raw[len(raw)-288:]
	}
	end := now.Truncate(time.Minute).Unix()
	var last int64
	if len(h.samples) > 0 {
		last = h.samples[len(h.samples)-1].At
	}
	started := last != 0
	for i, rate := range raw {
		at := end - int64(len(raw)-1-i)*60
		if rate < 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
			continue
		}
		// Discard only the initial stock padding; retain real zero-rate periods
		// after the first activity, including stops and failed pool connections.
		if !started && rate == 0 {
			continue
		}
		started = true
		if at < last {
			continue
		}
		if at == last && len(h.samples) > 0 {
			h.samples[len(h.samples)-1].Rate = rate
		} else {
			h.samples = append(h.samples, hashrateSample{at, rate})
			last = at
		}
	}
	h.prune(now)
	if h.path != "" {
		return atomicJSON(h.path, h.samples)
	}
	return nil
}

func (h *hashrateHistory) snapshot(now time.Time) []hashrateSample {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.prune(now)
	return append([]hashrateSample{}, h.samples...)
}

// Keep one sample per minute, but publish transitions to/from zero immediately.
// Do not rewrite the RAM-backed file for every three-second safety observation.
func (h *hashrateHistory) record(now time.Time, rate float64) error {
	if rate < 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.prune(now)
	at := now.Truncate(time.Minute).Unix()
	if n := len(h.samples); n > 0 {
		last := &h.samples[n-1]
		if at < last.At {
			return nil
		}
		if at == last.At {
			if last.Rate == rate || (last.Rate > 0 && rate > 0) {
				return nil
			}
			last.Rate = rate
		} else {
			h.samples = append(h.samples, hashrateSample{at, rate})
		}
	} else {
		h.samples = append(h.samples, hashrateSample{at, rate})
	}
	h.prune(now)
	if h.path != "" {
		return atomicJSON(h.path, h.samples)
	}
	return nil
}

func (h *hashrateHistory) empty() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.samples) == 0
}

// Correct only a known continuously stopped interval, including samples created
// by older collectors that kept timestamping a frozen native history response.
func (h *hashrateHistory) zeroSince(now, since time.Time) error {
	if since.IsZero() || since.After(now) {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.prune(now)
	start := max(since.Truncate(time.Minute).Unix(), now.Truncate(time.Minute).Unix()-1439*60)
	keep := 0
	for keep < len(h.samples) && h.samples[keep].At < start {
		keep++
	}
	h.samples = h.samples[:keep]
	for at := start; at <= now.Truncate(time.Minute).Unix(); at += 60 {
		h.samples = append(h.samples, hashrateSample{at, 0})
	}
	h.prune(now)
	if h.path != "" {
		return atomicJSON(h.path, h.samples)
	}
	return nil
}

// Caller receives a value only for a known stop/startup or fresh telemetry.
// In particular, the cached device snapshot retained by a stop is not live work.
func (c *Controller) historyRate(now time.Time) (float64, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.halted || c.startupPhase() != "" {
		return 0, true
	}
	if c.device == nil || c.lastGood.IsZero() || now.Sub(c.lastGood) > 15*time.Second {
		return 0, false
	}
	rate, ok := c.device["hashrate"].(float64)
	return rate, ok && rate >= 0 && !math.IsNaN(rate) && !math.IsInf(rate, 0)
}

func (c *Controller) collectHistory(now time.Time) error {
	rate, known := c.historyRate(now)
	if known && rate > 0 && c.history.empty() {
		// Import the native backlog once, only while live telemetry proves activity.
		// Its last bucket may be stale; the current bucket always uses live telemetry.
		b, err := c.native("GET", "/cpb/hshistory", nil, false)
		var values []float64
		rate, known = c.historyRate(now)
		if err == nil && json.Unmarshal(b, &values) == nil && len(values) > 1 && known && rate > 0 {
			if err = c.history.merge(now.Add(-time.Minute), values[:len(values)-1]); err != nil {
				return err
			}
		}
	}
	if known {
		return c.history.record(now, rate)
	}
	return nil
}
