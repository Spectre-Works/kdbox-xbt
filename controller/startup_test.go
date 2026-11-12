package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestColdXBTStartupUsesSeparateDeadlines(t *testing.T) {
	start := time.Now()
	c := Controller{firmware: "2.2.1", algorithm: 1, lastGood: start, device: map[string]any{"accepted": 5}}
	c.beginStockStartup(start)
	if c.startup.phase() != "fan-calibration" || c.device != nil || !c.lastGood.IsZero() {
		t.Fatal("cold startup exposed telemetry from the previous run")
	}
	engineStarted := start.Add(146 * time.Second)
	if waiting, err := c.startup.advance(engineStarted, false); err != nil || !waiting {
		t.Fatalf("stopped during the measured stock calibration time: waiting=%t, err=%v", waiting, err)
	}
	if waiting, err := c.startup.advance(engineStarted, true); err != nil || waiting {
		t.Fatalf("failed to enter engine startup: waiting=%t, err=%v", waiting, err)
	}
	if c.startup.phase() != "first-readings" || !c.startup.readingsUntil.Equal(engineStarted.Add(25*time.Second)) {
		t.Fatal("first-reading deadline must begin when the engine is seen")
	}
}

func TestClearStaleMinerPIDPreservesLiveProcess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "xminerd.pid")
	proc := filepath.Join(dir, "proc")
	if err := os.MkdirAll(filepath.Join(proc, "1234"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("1234\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := clearStaleMinerPID(path, proc); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("live PID was removed", err)
	}
	if err := os.Remove(filepath.Join(proc, "1234")); err != nil {
		t.Fatal(err)
	}
	if err := clearStaleMinerPID(path, proc); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("stale PID was retained", err)
	}
}

func TestStockCalibrationCannotWaitIndefinitely(t *testing.T) {
	start := time.Now()
	c := Controller{firmware: "2.2.1", algorithm: 1}
	c.beginStockStartup(start)
	if waiting, err := c.startup.advance(start.Add(stockFanCalibrationLimit), false); err == nil || waiting {
		t.Fatal("calibration timed out without requesting a stop")
	}
	c.algorithm = 0
	c.beginStockStartup(start)
	if c.startup.phase() != "" {
		t.Fatal("XBT startup policy applied to KDA")
	}
}
