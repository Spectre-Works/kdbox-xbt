package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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

func TestInitialFanSpinupOutlastsTwoSecondRPMRecheck(t *testing.T) {
	start := time.Now()
	c := Controller{firmware: "2.2.1", algorithm: 1}
	c.beginStockStartup(start)
	engineStarted := start.Add(90 * time.Second)
	_, _ = c.startup.advance(engineStarted, true)
	deadline := c.startup.fanSpinupUntil
	for _, sample := range []struct {
		seconds int
		rpm     [2]int
		waiting bool
	}{{0, [2]int{480, 480}, true}, {2, [2]int{1800, 1740}, true}, {6, [2]int{3300, 2700}, true}, {9, [2]int{4800, 4740}, false}} {
		waiting, err := c.startup.checkFans(engineStarted.Add(time.Duration(sample.seconds)*time.Second), func(i int) (int, error) { return sample.rpm[i], nil })
		if err != nil || waiting != sample.waiting {
			t.Fatalf("at %ds: waiting=%t err=%v", sample.seconds, waiting, err)
		}
		if waiting && !c.startup.fanSpinupUntil.Equal(deadline) {
			t.Fatal("startup polling extended the fan deadline")
		}
	}
	if !c.startup.fanSpinupUntil.IsZero() || c.startup.phase() != "first-readings" {
		t.Fatal("fan readiness lost the independent chip initialization phase")
	}
}

func TestStartupStalledOrUnreadableFanStops(t *testing.T) {
	for _, tc := range []struct {
		name    string
		seconds int
		rpm     int
		err     error
	}{{"stalled", 10, 0, nil}, {"slow", 10, 2940, nil}, {"sensor-error", 0, 0, errors.New("sensor")}, {"negative", 0, -1, nil}} {
		for fan := 0; fan < 2; fan++ {
			t.Run(tc.name+string(rune('1'+fan)), func(t *testing.T) {
				now := time.Now()
				s := engineStartup{fanSpinupUntil: now.Add(initialFanSpinupLimit)}
				waiting, err := s.checkFans(now.Add(time.Duration(tc.seconds)*time.Second), func(i int) (int, error) {
					if i == fan {
						return tc.rpm, tc.err
					}
					return 4800, nil
				})
				if waiting || err == nil || !strings.Contains(err.Error(), "Fan "+string(rune('1'+fan))) {
					t.Fatalf("fault did not identify the fan: waiting=%t err=%v", waiting, err)
				}
			})
		}
	}
}

func TestFanCannotReuseStartupAllowanceAfterBecomingHealthy(t *testing.T) {
	for _, healthy := range [][2]int{{4800, 480}, {4800, 4800}} {
		now := time.Now()
		s := engineStartup{fanSpinupUntil: now.Add(initialFanSpinupLimit)}
		if _, err := s.checkFans(now, func(i int) (int, error) { return healthy[i], nil }); err != nil {
			t.Fatal(err)
		}
		waiting, err := s.checkFans(now.Add(time.Second), func(int) (int, error) { return 0, nil })
		if waiting || err == nil {
			t.Fatal("a proven fan stall received a new startup allowance")
		}
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
