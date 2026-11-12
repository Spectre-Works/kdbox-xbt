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
