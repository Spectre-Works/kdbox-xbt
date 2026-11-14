package main

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type miningResetJournal struct {
	Version    int    `json:"version"`
	Failed     bool   `json:"failed,omitempty"`
	OwnerReset bool   `json:"owner_reset,omitempty"`
	Phase      string `json:"phase,omitempty"`
	BootID     string `json:"boot_id,omitempty"`
}

func readResetJournal(dir string) (miningResetJournal, error) {
	f, err := os.Open(filepath.Join(dir, "mining-reset.json"))
	if err != nil {
		return miningResetJournal{}, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, 16385))
	var j miningResetJournal
	if err != nil || len(b) > 16384 || json.Unmarshal(b, &j) != nil {
		return j, errors.New("invalid reset journal")
	}
	if j.Version == 1 && !j.OwnerReset && j.Phase == "" {
		return j, nil
	}
	if j.Version == 2 && j.OwnerReset && j.BootID != "" && (j.Phase == "owner" || j.Phase == "reboot" || j.Phase == "reboot-requested") {
		return j, nil
	}
	return j, errors.New("unsupported reset journal")
}

func (c *Controller) ownerResetStatus() (bool, bool) {
	j, err := readResetJournal(c.dir)
	return err == nil && j.OwnerReset, err == nil && j.OwnerReset && (j.Phase == "reboot" || j.Phase == "reboot-requested")
}

func (c *Controller) miningResetState() (bool, bool) {
	j, err := readResetJournal(c.dir)
	if os.IsNotExist(err) {
		return false, false
	}
	return true, c.resetBlocked || err != nil || j.Failed
}

func (c *Controller) resetOwnerSettings() error {
	if c.firmware != "2.2.1" || c.ownerReset == nil || c.ownerReboot == nil {
		return errors.New("factory reset requires the pinned 2.2.1 device adapter")
	}
	if c.pending != nil || c.networkPending != nil {
		return errors.New("finish the current adjustment before factory reset")
	}
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".kdbox-update.") {
			return errors.New("finish the software update before factory reset")
		}
	}
	bootID := currentBootID()
	if bootID == "" {
		return errors.New("could not verify the boot session")
	}
	if err = atomicJSON(filepath.Join(c.dir, "mining-reset.json"), miningResetJournal{Version: 2, OwnerReset: true, Phase: "owner", BootID: bootID}); err != nil {
		return err
	}
	c.resetBlocked = false
	c.stop(ownerResetPause)
	return nil
}

func (c *Controller) resetMiningSettings() error {
	if c.pending != nil || c.networkPending != nil {
		return errors.New("finish the current performance or network adjustment before resetting mining settings")
	}
	// The safety loop advances one pool at a time; this request returns promptly.
	// Write ahead before changing pause state: if storage rejects the journal,
	// the request has made no change. Once present, it independently blocks startup.
	if err := atomicJSON(filepath.Join(c.dir, "mining-reset.json"), miningResetJournal{Version: 1}); err != nil {
		return err
	}
	c.resetBlocked = false
	c.stop(settingsResetPause) // Keeps the first genuine fault, if one is present.
	if _, _, err := c.readPause(); err != nil {
		c.resetBlocked = true
		_ = atomicJSON(filepath.Join(c.dir, "mining-reset.json"), miningResetJournal{Version: 1, Failed: true})
		return err
	}
	return nil
}

// Bounded, idempotent journal recovery. Ambiguous native failures require an
// explicit retry; a failed DELETE is never blindly repeated in the background.
func (c *Controller) advanceMiningReset() {
	path := filepath.Join(c.dir, "mining-reset.json")
	_, failed := c.miningResetState()
	if failed {
		return
	}
	err := c.miningResetStep()
	if err != nil {
		c.resetBlocked = true
		j, readErr := readResetJournal(c.dir)
		if readErr == nil {
			j.Failed = true
			_ = atomicJSON(path, j)
		}
		c.stop("Settings reset needs inspection: " + err.Error())
	}
}

func (c *Controller) miningResetStep() error {
	j, err := readResetJournal(c.dir)
	if err != nil {
		return err
	}
	if j.OwnerReset {
		return c.ownerResetStep(j)
	}
	if _, _, err := c.readPause(); err != nil {
		return err
	}
	pools, err := c.savedPools()
	if err != nil {
		return err
	}
	// Persist the inspection latch before any write. Power loss or an ambiguous
	// response cannot turn an unfinished native mutation into an automatic retry.
	if err = atomicJSON(filepath.Join(c.dir, "mining-reset.json"), miningResetJournal{Version: 1, Failed: true}); err != nil {
		return err
	}
	if len(pools) > 0 {
		if _, err = c.native("PUT", "/mcb/delpool", pools[0], true); err != nil {
			return err
		}
		remaining, e := c.savedPools()
		if e != nil || len(remaining) >= len(pools) {
			return errors.New("could not confirm pool removal")
		}
		return atomicJSON(filepath.Join(c.dir, "mining-reset.json"), miningResetJournal{Version: 1})
	}
	cfg := c.cfg
	cfg.ChipTarget = chipTemperatureTarget
	cfg.AutomaticTuning = false
	if err = atomicJSON(filepath.Join(c.dir, "config.json"), cfg); err != nil {
		return err
	}
	c.cfg = cfg
	if err = atomicJSON(filepath.Join(c.dir, "approved.json"), defaultTuning); err != nil {
		return err
	}
	c.approved = defaultTuning
	if err = atomicJSON(c.algorithmPath(), defaultAlgorithm); err != nil {
		return err
	}
	c.algorithm = defaultAlgorithm
	c.algorithmQueued = true
	c.cooling = automaticCooling{}
	c.automatic = automaticPerformance{}
	c.device = nil
	return os.Remove(filepath.Join(c.dir, "mining-reset.json"))
}

func (c *Controller) ownerResetStep(j miningResetJournal) error {
	path := filepath.Join(c.dir, "mining-reset.json")
	switch j.Phase {
	case "owner":
		if c.ownerReset == nil {
			return errors.New("factory reset adapter is unavailable")
		}
		if _, _, err := c.readPause(); err != nil {
			return err
		}
		j.Failed = true
		if err := atomicJSON(path, j); err != nil {
			return err
		}
		if err := c.ownerReset(); err != nil {
			return err
		}
		// Owner reset deliberately clears personalization and benign inspection
		// pauses. Genuine/unknown faults remain latched, with their original record.
		r, _, err := c.readPause()
		if err != nil {
			return err
		}
		if r.Kind == "pool-setup" || r.Kind == "recovery" || r.Kind == "software-update" {
			if err = atomicJSON(filepath.Join(c.dir, "halted.json"), map[string]string{"reason": ownerResetPause, "kind": "pool-setup", "at": time.Now().UTC().Format(time.RFC3339)}); err != nil {
				return err
			}
			c.message = ownerResetPause
		}
		c.cfg = Config{AdminPassword: factoryWebPassword, ChipTarget: chipTemperatureTarget}
		c.approved = defaultTuning
		c.algorithm = defaultAlgorithm
		c.algorithmQueued = true
		c.cooling = automaticCooling{}
		c.automatic = automaticPerformance{}
		c.device = nil
		c.token = ""
		c.sessions = map[string]Session{}
		c.autoResumeBlocked = false
		j.Phase = "reboot"
		j.Failed = false
		return atomicJSON(path, j)
	case "reboot":
		if c.ownerReboot == nil {
			return errors.New("reboot adapter is unavailable")
		}
		j.Phase = "reboot-requested"
		if err := atomicJSON(path, j); err != nil {
			return err
		}
		return c.ownerReboot()
	case "reboot-requested":
		return nil // Never blindly repeat an ambiguous reboot request.
	default:
		return errors.New("unsupported factory reset phase")
	}
}
