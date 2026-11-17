package main

import (
	"errors"
	"os"
	"strings"
	"time"
)

const stockMinerLog = "/backup/bfgminer/trash/minerlog"
const stockRecoveryLimit = 15 * time.Second
const stockRecoveryCooldown = time.Minute

// Follow only newly appended, bounded log data. Existing text, rotation,
// truncation and unreadable/oversized gaps cannot authorize a recovery.
type stockRecovery struct {
	file                      os.FileInfo
	offset                    int64
	partial                   string
	event, until, lastAttempt time.Time
}

func (s *stockRecovery) observe(now time.Time, path string) {
	f, err := os.Open(path)
	if err != nil {
		s.file = nil
		s.event = time.Time{}
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		s.file = nil
		s.event = time.Time{}
		return
	}
	if s.file == nil || !os.SameFile(s.file, info) || info.Size() < s.offset || info.Size()-s.offset > 16384 {
		s.file = info
		s.offset = info.Size()
		s.partial = ""
		s.event = time.Time{}
		return
	}
	count := info.Size() - s.offset
	if count == 0 {
		return
	}
	data := make([]byte, int(count))
	if _, err = f.ReadAt(data, s.offset); err != nil {
		s.file = nil
		s.event = time.Time{}
		return
	}
	s.offset = info.Size()
	lines := strings.Split(s.partial+string(data), "\n")
	s.partial = lines[len(lines)-1]
	if len(s.partial) > 512 {
		s.partial = ""
	}
	for _, line := range lines[:len(lines)-1] {
		if strings.HasSuffix(strings.TrimSpace(line), "] C0: gskb_reinit") {
			s.event = now
		}
	}
}

// Called after raw temperatures have been checked. Returning true holds normal
// cooling/optimization while stock restores its own setpoint; no setpoint writes.
func (s *stockRecovery) check(now time.Time, expected, reported Tuning, verified bool) (waiting, completed bool, err error) {
	if !s.until.IsZero() {
		if !now.Before(s.until) {
			return false, false, errors.New("Stock hashboard recovery did not restore the managed clock before its deadline")
		}
		if !verified {
			return true, false, nil
		}
		if reported.Millivolts != expected.Millivolts || reported.Clock < 50 || reported.Clock > expected.Clock {
			return false, false, errors.New("Clock or voltage is outside the stock recovery envelope")
		}
		if reported == expected {
			s.until = time.Time{}
			s.event = time.Time{}
			return false, true, nil
		}
		return true, false, nil
	}
	if !verified || reported.Millivolts != expected.Millivolts || reported.Clock < 50 || reported.Clock >= expected.Clock {
		return false, false, nil
	}
	if s.event.IsZero() || now.Sub(s.event) < 0 || now.Sub(s.event) > 6*time.Second {
		return false, false, nil
	}
	if !s.lastAttempt.IsZero() && now.Sub(s.lastAttempt) < stockRecoveryCooldown {
		return false, false, errors.New("Stock hashboard recovery repeated too soon; inspect the miner")
	}
	s.until = s.event.Add(stockRecoveryLimit)
	s.lastAttempt = now
	s.event = time.Time{}
	return true, false, nil
}

func (c *Controller) startupPhase() string {
	if !c.stockRecovery.until.IsZero() {
		return "stock-recovery"
	}
	return c.startup.phase()
}

// An interrupted automatic qualification cannot commit mixed observations from
// before and after a hardware reset. Established tuning remains unchanged.
func (c *Controller) finishStockRecovery() bool {
	c.automatic = automaticPerformance{}
	c.cooling = automaticCooling{percent: 100}
	if c.pending != nil {
		c.rejectPerformance("Stock hashboard recovery interrupted automatic qualification; restored previous settings")
		return true
	}
	c.message = "KD BOX safeguards active"
	return false
}
