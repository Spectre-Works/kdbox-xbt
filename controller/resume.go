package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func (c *Controller) nativeAlgorithm() (map[string]json.RawMessage, int, error) {
	body, err := c.native("GET", "/mcb/algosetting", nil, false)
	if err != nil {
		return nil, 0, err
	}
	var menu map[string]json.RawMessage
	var id *int
	if json.Unmarshal(body, &menu) != nil || json.Unmarshal(menu["algo_select"], &id) != nil || id == nil || !validAlgorithm(*id) {
		return nil, 0, errors.New("stock firmware returned an invalid algorithm")
	}
	return menu, *id, nil
}

func validAlgorithm(id int) bool { return id == 0 || id == 1 }

const defaultAlgorithm = 1 // Bitcoin (XBT), called blake2b(SC) by stock firmware.

func (c *Controller) algorithmPath() string { return filepath.Join(c.dir, "algorithm.json") }

func (c *Controller) resumeMining() error {
	return c.resumeMiningFromPause(nil)
}

func (c *Controller) resumeMiningFromPause(expected []byte) error {
	if !c.halted {
		return nil
	}
	if _, err := os.Stat(filepath.Join(c.dir, "mining-reset.json")); !os.IsNotExist(err) {
		return errors.New("finish the settings reset before resuming")
	}
	_, current, err := c.readPause()
	if err != nil {
		return err
	}
	if expected != nil && !bytes.Equal(current, expected) {
		return errors.New("pause changed; inspection required")
	}
	if c.pending != nil {
		if err := c.rollback("Recovered the interrupted automatic adjustment"); err != nil {
			return err
		}
	}
	if c.firmware == "2.2.1" && c.algorithm == 1 {
		if err := clearStaleMinerPID("/var/run/xminerd.pid", "/proc"); err != nil {
			return fmt.Errorf("cannot verify the stock mining PID file: %w", err)
		}
	}
	if err := c.apply(c.approved); err != nil {
		return err
	}
	if expected != nil {
		_, current, err = c.readPause()
		if err != nil || !bytes.Equal(current, expected) {
			return errors.New("pause changed; inspection required")
		}
	}
	if err := os.Remove(filepath.Join(c.dir, "halted.json")); err != nil {
		return err
	}
	c.halted = false
	c.autoResumeBlocked = false
	c.lastGood = time.Now()
	c.message = "Managed settings restored"
	if err := c.reapplyAlgorithm(); err != nil {
		c.stop("Resume failed while restoring mining engine: " + err.Error())
		return err
	}
	c.beginStockStartup(time.Now())
	_ = exec.Command("/bin/sh", "-c", `for p in $(pidof xminerd); do kill -CONT "$p"; done`).Run()
	return nil
}

// The firmware may restart the engine even when the requested algorithm
// matches its setting, so callers run this only after clearing a safety stop.
func (c *Controller) reapplyAlgorithm() error {
	menu, _, err := c.nativeAlgorithm()
	if err != nil {
		return err
	}
	menu["algo_select"] = json.RawMessage(fmt.Sprint(c.algorithm))
	if _, err = c.native("PUT", "/mcb/algosetting", menu, true); err != nil {
		return err
	}
	_, actual, err := c.nativeAlgorithm()
	if err != nil {
		return err
	}
	if actual != c.algorithm {
		return errors.New("stock firmware did not retain the selected algorithm")
	}
	c.algorithmQueued = false
	return nil
}

func (c *Controller) restoreAlgorithm() error {
	_, native, err := c.nativeAlgorithm()
	if err != nil {
		return err
	}
	c.algorithm = native
	data, err := os.ReadFile(c.algorithmPath())
	if err == nil {
		var saved *int
		if json.Unmarshal(data, &saved) != nil || saved == nil || !validAlgorithm(*saved) {
			return errors.New("saved algorithm selection is invalid")
		}
		c.algorithm = *saved
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	} else {
		c.algorithm = defaultAlgorithm
		if err := atomicJSON(c.algorithmPath(), c.algorithm); err != nil {
			return err
		}
	}
	if c.halted {
		c.algorithmQueued = c.algorithm != native
		return nil
	}
	return c.reapplyAlgorithm()
}

func (c *Controller) chooseAlgorithm(id int) error {
	if !validAlgorithm(id) {
		return errors.New("select a supported mining algorithm")
	}
	if c.cfg.AutomaticTuning {
		return errors.New("turn off automatic performance before changing algorithms")
	}
	if c.pending != nil {
		return errors.New("automatic performance adjustment is in progress")
	}
	if c.halted {
		if err := atomicJSON(c.algorithmPath(), id); err != nil {
			return err
		}
		c.algorithm = id
		c.algorithmQueued = true
		return nil
	}
	token, err := c.login(c.cfg.AdminPassword)
	if err != nil {
		return err
	}
	c.token = token
	previous := c.algorithm
	c.algorithm = id
	if err = c.reapplyAlgorithm(); err != nil {
		c.algorithm = previous
		return err
	}
	if err = atomicJSON(c.algorithmPath(), id); err != nil {
		c.algorithm = previous
		if rollbackErr := c.reapplyAlgorithm(); rollbackErr != nil {
			c.stop("Could not recover the previous algorithm after a settings write failed")
		}
		return err
	}
	return nil
}
