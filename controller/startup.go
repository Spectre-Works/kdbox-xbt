package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	stockFanCalibrationLimit = 4 * time.Minute
	firstMiningReadingsLimit = 25 * time.Second
)

type engineStartup struct {
	calibrationUntil time.Time
	readingsUntil    time.Time
}

// Keep the longer calibration deadline separate from the time allowed to
// initialize sensors once a hashing process is present.
func (c *Controller) beginStockStartup(now time.Time) {
	c.startup = engineStartup{}
	c.miningSession = miningSession{}
	c.cooling = automaticCooling{}
	c.lastGood = time.Time{}
	c.device = nil
	if c.firmware != "2.2.1" || c.algorithm != 1 {
		return
	}
	c.startup.calibrationUntil = now.Add(stockFanCalibrationLimit)
	c.message = "Waiting for stock fan calibration before XBT starts"
}

func processRunning(names ...string) (bool, error) {
	output, err := exec.Command("pidof", names...).Output()
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	for _, pid := range strings.Fields(string(output)) {
		data, err := os.ReadFile(filepath.Join("/proc", pid, "stat"))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return false, err
		}
		_, tail, ok := strings.Cut(string(data), ") ")
		if !ok || tail == "" {
			return false, errors.New("invalid process state")
		}
		if tail[0] != 'Z' {
			return true, nil
		}
	}
	return false, nil
}

// The stock supervisor treats the presence of this file as proof that its
// engine is still up, even when a safety stop killed the engine first.
func clearStaleMinerPID(path, procDir string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > 32 {
		return errors.New("invalid stock mining PID file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid < 2 {
		return errors.New("invalid stock mining PID")
	}
	if _, err := os.Stat(filepath.Join(procDir, strconv.Itoa(pid))); !os.IsNotExist(err) {
		return err
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// advance returns true while no engine is running and the stock supervisor can
// calibrate its fans. Once an engine is seen, this startup cannot reenter that phase.
func (s *engineStartup) advance(now time.Time, engineRunning bool) (bool, error) {
	if engineRunning {
		s.calibrationUntil = time.Time{}
		s.readingsUntil = now.Add(firstMiningReadingsLimit)
		return false, nil
	}
	if !now.Before(s.calibrationUntil) {
		return false, errors.New("XBT engine did not start before the stock fan calibration deadline")
	}
	return true, nil
}

func (s engineStartup) phase() string {
	if !s.calibrationUntil.IsZero() {
		return "fan-calibration"
	}
	if !s.readingsUntil.IsZero() {
		return "first-readings"
	}
	return ""
}
