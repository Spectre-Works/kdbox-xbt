package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const settingsResetPause = "Mining settings reset; add a pool before mining resumes"
const recoveryPause = "Recovered after missing installation settings; review configuration before resuming"
const softwareUpdatePause = "Paused for a software update; review settings before resuming"
const ownerResetPause = "Factory reset; add a pool before mining resumes"

type pauseRecord struct {
	Reason string `json:"reason"`
	Kind   string `json:"kind,omitempty"`
}

func pauseKind(reason string) string {
	switch reason {
	case poolSetupPause, settingsResetPause, ownerResetPause:
		return "pool-setup"
	case recoveryPause:
		return "recovery"
	case softwareUpdatePause:
		return "software-update"
	default:
		return "fault"
	}
}

func (c *Controller) readPause() (pauseRecord, []byte, error) {
	f, err := os.Open(filepath.Join(c.dir, "halted.json"))
	if err != nil {
		return pauseRecord{}, nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, (64<<10)+1))
	var record pauseRecord
	if err != nil || len(b) > 64<<10 || json.Unmarshal(b, &record) != nil || record.Reason == "" {
		return record, nil, errors.New("paused state is unreadable; inspection required")
	}
	if record.Kind == "" {
		record.Kind = pauseKind(record.Reason)
	}
	return record, b, nil
}

func (c *Controller) pauseStatus() (string, bool) {
	if !c.halted {
		return "", false
	}
	r, _, err := c.readPause()
	if err != nil {
		return "fault", false
	}
	// An explicit fault must never be reclassified by its display wording.
	automatic := r.Kind == "pool-setup" && pauseKind(r.Reason) == "pool-setup" && !c.autoResumeBlocked
	return r.Kind, automatic
}

// Only an owned no-pool pause is eligible. Every unknown/inspection-required
// stop remains latched, including after a pool is added or an algorithm changes.
func (c *Controller) tryResumePoolSetup() {
	_, automatic := c.pauseStatus()
	if !automatic || c.pending != nil || c.networkPending != nil {
		return
	}
	if c.firmware != "2.2.1" || c.cfg.validatePerformance() != nil || c.approved.Validate(c.approved) != nil || !validAlgorithm(c.algorithm) || (c.cfg.AutomaticTuning && c.algorithm != 1) {
		return
	}
	if _, err := os.Stat(filepath.Join(c.dir, "mining-reset.json")); !os.IsNotExist(err) {
		return
	}
	pools, err := c.savedPools()
	if err != nil || len(pools) == 0 {
		return
	}
	for _, p := range pools {
		address, _ := p["url"].(string)
		worker, _ := p["user"].(string)
		if _, err = normalizePool(address, worker); err != nil {
			return
		}
	}
	_, expected, err := c.readPause()
	if err != nil {
		return
	}
	if err = c.resumeMiningFromPause(expected); err == nil {
		return
	}
	c.autoResumeBlocked = true
	// A failed startup is a fault, not an endlessly retried setup condition.
	record, current, readErr := c.readPause()
	if readErr == nil && bytes.Equal(current, expected) && record.Kind == "pool-setup" {
		if e := atomicJSON(filepath.Join(c.dir, "halted.json"), map[string]any{
			"reason": fmt.Sprintf("Automatic startup failed; inspect before resuming: %v", err),
			"kind":   "fault", "previous_reason": record.Reason,
		}); e != nil {
			c.message = "Automatic startup failed; cannot persist fault: " + e.Error()
		}
	}
	c.stop("Automatic startup failed; inspect before resuming: " + err.Error())
}
