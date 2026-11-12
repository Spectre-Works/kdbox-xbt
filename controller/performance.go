package main

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"time"
)

// Factory administration through the owner's private runtime directory and
// SIGHUP, never through the customer HTTP API. The ceiling still cannot exceed
// the stock-envelope hard bound, and raising it does not apply a clock directly.
func (c *Controller) reloadFactoryClockLimit() error {
	path := filepath.Join(c.dir, "clock-limit-request.json")
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || info.Size() > 1024 {
		return errors.New("invalid private factory clock request")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var q struct {
		MaximumClock int `json:"maximum_clock"`
	}
	if json.Unmarshal(b, &q) != nil {
		return errors.New("invalid factory clock request")
	}
	limits := performanceLimits{430, q.MaximumClock}
	cfg := c.cfg
	cfg.AutomaticLimits = &limits
	if err := cfg.validatePerformance(); err != nil {
		return err
	}
	if c.cfg.AutomaticTuning && c.pending != nil && c.pending.Candidate.Clock > limits.MaximumClock {
		if err := c.rollback("Returning to the previous point for the qualified clock limit"); err != nil {
			return err
		}
	}
	if err := atomicJSON(filepath.Join(c.dir, "config.json"), cfg); err != nil {
		return err
	}
	c.cfg = cfg
	if c.cfg.AutomaticTuning && c.approved.Clock > limits.MaximumClock {
		next := c.approved
		next.Clock = limits.MaximumClock
		if err := c.startPerformanceChange(next, false, "Applying the qualified clock limit"); err != nil {
			c.stop("Could not apply the qualified clock limit")
			return err
		}
	}
	return os.Remove(path)
}

type performanceLimits struct {
	MinimumMillivolts int `json:"minimum_millivolts"`
	MaximumClock      int `json:"maximum_clock"`
}

func (cfg Config) performanceLimits() performanceLimits {
	if cfg.AutomaticLimits != nil {
		limits := *cfg.AutomaticLimits
		if limits.MinimumMillivolts == 425 {
			limits.MinimumMillivolts = 430
		}
		return limits
	}
	return performanceLimits{430, defaultTuning.Clock}
}

func (cfg Config) validatePerformance() error {
	l := cfg.performanceLimits()
	if l.MinimumMillivolts != 430 || l.MaximumClock < 500 || l.MaximumClock > maximumAutomaticClock || l.MaximumClock%5 != 0 {
		return errors.New("invalid automatic performance limits")
	}
	return nil
}

func initialPerformancePoint(cfg Config, saved Tuning) Tuning {
	if !cfg.AutomaticTuning || saved.automaticBounds() != nil {
		saved = defaultTuning
	}
	if cfg.AutomaticTuning {
		saved.Clock = min(saved.Clock, cfg.performanceLimits().MaximumClock)
	}
	return saved
}

type performanceReading struct {
	Filtered, Peak, HW, Nonces, Accepted float64
}

func performanceCounters(d map[string]any) (hw, nonces, accepted float64, valid bool) {
	values := make([]float64, 0, 3)
	for _, name := range []string{"hwerrors", "nonces", "accepted"} {
		v, ok := d[name].(float64)
		if !ok || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
			return 0, 0, 0, false
		}
		values = append(values, v)
	}
	return values[0], values[1], values[2], true
}

// A single invalid nonce is not a fault. Automatic mode backs off for either
// 20 new errors in a five-minute window or >=5 above 0.1% of new nonces.
// Counter resets cannot manufacture a rate. This policy never requests a stop.
func excessiveHardwareErrors(oldHW, oldNonces, hw, nonces float64) bool {
	rate, valid := errorRate(oldHW, oldNonces, hw, nonces)
	errors := hw - oldHW
	return hw >= oldHW && nonces >= oldNonces && (errors >= 20 || (valid && errors >= 5 && rate > 0.1))
}

type performanceDecision struct {
	Setting Tuning
	Reason  string
	Verify  bool
	Quality bool
	Stop    bool
}

type automaticPerformance struct {
	since, hotSince, rawHotSince, coolSince time.Time
	baseline, qualityBaseline               performanceReading
	qualitySince                            time.Time
	message                                 string
}

// Frequency handles the thermal budget at fixed voltage. Each increase must
// produce accepted work within the bounded invalid-nonce budget before retention.
func (a *automaticPerformance) next(now time.Time, target float64, current Tuning, limits performanceLimits, r performanceReading) performanceDecision {
	if a.since.IsZero() || r.Nonces < a.baseline.Nonces || r.Accepted < a.baseline.Accepted || r.HW < a.baseline.HW {
		*a = automaticPerformance{since: now, baseline: r, qualityBaseline: r, qualitySince: now, message: "Waiting for stable mining"}
	}
	rate, valid := errorRate(a.baseline.HW, a.baseline.Nonces, r.HW, r.Nonces)

	if r.Filtered > target+1.5 {
		if a.hotSince.IsZero() {
			a.hotSince = now
		}
	} else if r.Filtered <= target+0.5 {
		a.hotSince = time.Time{}
	}
	if r.Peak >= target+5 {
		if a.rawHotSince.IsZero() {
			a.rawHotSince = now
		}
	} else {
		a.rawHotSince = time.Time{}
	}
	tooHot := (!a.hotSince.IsZero() && now.Sub(a.hotSince) >= 30*time.Second) ||
		(!a.rawHotSince.IsZero() && now.Sub(a.rawHotSince) >= 9*time.Second)
	if tooHot {
		if current.Clock == 500 {
			return performanceDecision{Stop: true, Reason: "Cannot maintain the temperature limit at minimum performance and full cooling"}
		}
		next := current
		next.Clock = max(500, current.Clock-25)
		return performanceDecision{Setting: next, Reason: "Reducing clock to meet the temperature limit"}
	}
	if a.qualitySince.IsZero() {
		a.qualitySince, a.qualityBaseline = a.since, a.baseline
	}
	if excessiveHardwareErrors(a.qualityBaseline.HW, a.qualityBaseline.Nonces, r.HW, r.Nonces) {
		a.qualitySince, a.qualityBaseline = now, r
		if current.Clock > 500 {
			next := current
			next.Clock = max(500, current.Clock-25)
			return performanceDecision{Setting: next, Quality: true, Reason: "Reducing clock after excessive invalid nonces"}
		}
		a.message = "Invalid nonce rate elevated at minimum clock"
	}
	if now.Sub(a.qualitySince) >= 5*time.Minute {
		a.qualitySince, a.qualityBaseline = now, r
	}
	if r.Filtered < target-2 {
		if a.coolSince.IsZero() {
			a.coolSince = now
		}
	} else {
		a.coolSince = time.Time{}
	}
	healthy := now.Sub(a.since) >= 5*time.Minute && r.Nonces-a.baseline.Nonces >= 1000 &&
		r.Accepted > a.baseline.Accepted && valid && rate <= 0.1
	if !healthy {
		a.message = "Waiting for stable mining"
		return performanceDecision{}
	}
	if !a.coolSince.IsZero() && now.Sub(a.coolSince) >= 2*time.Minute && current.Clock < limits.MaximumClock {
		next := current
		next.Clock = min(limits.MaximumClock, current.Clock+10)
		return performanceDecision{Setting: next, Verify: true, Reason: "Using available cooling headroom"}
	}
	a.message = "Maintaining the temperature limit"
	return performanceDecision{}
}

func (c *Controller) activeTuning() Tuning {
	if c.pending != nil {
		return c.pending.Candidate
	}
	return c.approved
}

func (c *Controller) performanceStatus() map[string]any {
	detail := "Using SPECTRE defaults"
	if c.cfg.AutomaticTuning {
		detail = c.automatic.message
		if detail == "" {
			detail = "Waiting for stable mining"
		}
		if c.pending != nil {
			detail = "Checking automatic settings"
		}
	}
	return map[string]any{"automatic": c.cfg.AutomaticTuning, "default": defaultTuning,
		"active": c.activeTuning(), "adjusting": c.pending != nil, "detail": detail,
		"available": c.firmware == "2.2.1" && c.algorithm == 1, "limits": c.cfg.performanceLimits()}
}

func (c *Controller) startPerformanceChange(next Tuning, verify bool, reason string) error {
	if err := next.automaticBounds(); err != nil {
		return err
	}
	if next != defaultTuning && next.Clock > c.cfg.performanceLimits().MaximumClock {
		return errors.New("clock exceeds this unit's qualified limit")
	}
	if !verify && next != defaultTuning && (next.Millivolts != c.approved.Millivolts || next.Clock > c.approved.Clock) {
		return errors.New("automatic stability verification is required for this adjustment")
	}
	if next != defaultTuning || verify {
		if err := next.Validate(c.approved); err != nil {
			return err
		}
	}
	if c.pending != nil {
		return errors.New("an automatic performance adjustment is already in progress")
	}
	p := Pending{Previous: c.approved, Candidate: next}
	if err := atomicJSON(filepath.Join(c.dir, "pending.json"), p); err != nil {
		return err
	}
	c.pending = &p
	c.started = time.Time{}
	c.deadline = time.Now().Add(stockFanCalibrationLimit + firstMiningReadingsLimit + 10*time.Minute)
	var err error
	if c.cfg.AutomaticTuning && !c.halted && c.device != nil && c.startup.phase() == "" && c.setClock != nil && next.Millivolts == c.approved.Millivolts {
		c.baselineHW, c.baselineNonces, c.baselineAccepted = number(c.device["hwerrors"]), number(c.device["nonces"]), number(c.device["accepted"])
		err = c.setClock(next.Clock)
		c.started = time.Now()
		c.deadline = c.started.Add(10 * time.Minute)
		c.automatic = automaticPerformance{}
		if err != nil {
			// Re-establish the requested bounded point through the full stock
			// profile path if the live setter could not be verified.
			c.started = time.Time{}
			c.deadline = time.Now().Add(stockFanCalibrationLimit + firstMiningReadingsLimit + 10*time.Minute)
			err = c.apply(next)
		}
	} else {
		err = c.apply(next)
	}
	if err != nil {
		_ = c.rollback("Automatic adjustment failed; restored the previous operating point")
		return err
	}
	if !verify {
		if err := c.commitPerformance(); err != nil {
			c.stop("Cannot save managed performance settings")
			return err
		}
	}
	c.message = reason
	return nil
}

func (c *Controller) commitPerformance() error {
	next := c.pending.Candidate
	if err := atomicJSON(filepath.Join(c.dir, "approved.json"), next); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(c.dir, "pending.json")); err != nil {
		return err
	}
	c.approved, c.pending = next, nil
	if !c.started.IsZero() {
		c.automatic.since = c.started
		c.automatic.baseline = performanceReading{HW: c.baselineHW, Nonces: c.baselineNonces, Accepted: c.baselineAccepted}
	} else {
		c.automatic = automaticPerformance{}
	}
	return nil
}

func (c *Controller) rejectPerformance(reason string) {
	p := *c.pending
	l := c.cfg.performanceLimits()
	l.MinimumMillivolts = 430
	if p.Candidate.Clock > p.Previous.Clock {
		l.MaximumClock = min(l.MaximumClock, p.Previous.Clock)
	}
	cfg := c.cfg
	cfg.AutomaticLimits = &l
	if err := atomicJSON(filepath.Join(c.dir, "config.json"), cfg); err != nil {
		c.stop("Cannot preserve automatic recovery limits")
		return
	}
	c.cfg = cfg
	_ = c.rollback(reason)
}

// Called only after complete, raw temperature checks and independent fan checks.
func (c *Controller) performanceSafety(now time.Time, hardwareVerified bool) {
	if !hardwareVerified {
		c.fans(100)
		if c.lastHardware.IsZero() {
			c.lastHardware = now
		}
		if !now.Before(c.engineGrace) && now.Sub(c.lastHardware) > 10*time.Second {
			c.stop("Cannot verify the managed clock and voltage")
		}
		return
	}
	c.lastHardware = now
	d := c.device
	expected := c.activeTuning()
	reported := Tuning{Clock: int(number(d["clock"])), Millivolts: int(number(d["voltage"])*1000 + 0.5)}
	if reported.automaticBounds() != nil {
		c.stop("Clock or voltage is outside the managed performance limits")
		return
	}
	if reported != expected {
		if now.Before(c.engineGrace) {
			return
		}
		if c.pending != nil && reported == c.pending.Previous {
			c.rejectPerformance("The miner did not apply the automatic setting; restored the previous operating point")
			return
		}
		c.stop("Clock or voltage changed outside the managed performance settings")
		return
	}
	if !c.cfg.AutomaticTuning {
		return // Invalid nonces are normal telemetry, never a default-mode stop.
	}
	hw, nonces, accepted, countersValid := performanceCounters(d)
	if !countersValid {
		if c.pending != nil {
			c.rejectPerformance("Restored the previous clock because quality counters are unavailable")
		}
		c.automatic = automaticPerformance{message: "Waiting for mining quality counters"}
		return
	}
	r := performanceReading{c.cooling.peak, number(d["peak_temp"]), hw, nonces, accepted}
	if c.pending != nil {
		if c.started.IsZero() {
			c.started = now
			c.baselineHW, c.baselineNonces, c.baselineAccepted = 0, r.Nonces, r.Accepted
			c.deadline = now.Add(10 * time.Minute)
		}
		rate, valid := errorRate(c.baselineHW, c.baselineNonces, r.HW, r.Nonces)
		if excessiveHardwareErrors(c.baselineHW, c.baselineNonces, r.HW, r.Nonces) {
			c.rejectPerformance("Restored the previous clock after excessive invalid nonces")
			return
		}
		if r.HW < c.baselineHW || r.Nonces < c.baselineNonces || r.Accepted < c.baselineAccepted {
			c.rejectPerformance("Restored the previous clock after mining counters reset unexpectedly")
			return
		}
		if now.After(c.deadline) {
			c.rejectPerformance("Restored the previous settings because automatic settings could not be verified")
			return
		}
		thermal := c.automatic.next(now, float64(c.cfg.coolingTarget()), expected, c.cfg.performanceLimits(), r)
		if thermal.Stop {
			c.stop(thermal.Reason)
			return
		}
		if thermal.Setting.Clock != 0 && thermal.Setting.Clock < expected.Clock {
			previous := c.pending.Previous
			if err := c.rollback("Reducing performance to meet the temperature limit"); err != nil {
				return
			}
			previous.Clock = max(500, min(previous.Clock, expected.Clock)-25)
			if err := c.startPerformanceChange(previous, false, thermal.Reason); err != nil {
				c.stop("Could not reduce automatic performance")
			}
			return
		}
		if now.Sub(c.started) >= 5*time.Minute && r.Nonces-c.baselineNonces >= 1000 &&
			r.Accepted > c.baselineAccepted && valid && rate <= 0.1 {
			if err := c.commitPerformance(); err != nil {
				c.stop("Cannot save stable automatic settings")
			}
		}
		return
	}
	decision := c.automatic.next(now, float64(c.cfg.coolingTarget()), c.approved, c.cfg.performanceLimits(), r)
	if decision.Stop {
		c.stop(decision.Reason)
		return
	}
	if decision.Setting != (Tuning{}) {
		if decision.Quality {
			cfg := c.cfg
			limits := cfg.performanceLimits()
			limits.MaximumClock = min(limits.MaximumClock, decision.Setting.Clock)
			cfg.AutomaticLimits = &limits
			if err := atomicJSON(filepath.Join(c.dir, "config.json"), cfg); err != nil {
				c.stop("Cannot preserve automatic recovery limits")
				return
			}
			c.cfg = cfg
		}
		if err := c.startPerformanceChange(decision.Setting, decision.Verify, decision.Reason); err != nil && !c.halted {
			c.stop("Automatic performance adjustment failed: " + err.Error())
		}
	}
}

func (c *Controller) setAutomaticPerformance(enabled bool) error {
	if enabled && (c.firmware != "2.2.1" || c.algorithm != 1) {
		return errors.New("automatic performance requires XBT on stock 2.2.1")
	}
	if c.cfg.AutomaticTuning == enabled {
		return nil
	}
	if c.pending != nil {
		if err := c.rollback("Automatic adjustment canceled"); err != nil {
			return err
		}
	}
	cfg := c.cfg
	cfg.AutomaticTuning = enabled
	if err := atomicJSON(filepath.Join(c.dir, "config.json"), cfg); err != nil {
		return err
	}
	c.cfg = cfg
	c.automatic = automaticPerformance{}
	if enabled && c.approved.Clock > cfg.performanceLimits().MaximumClock {
		next := c.approved
		next.Clock = cfg.performanceLimits().MaximumClock
		if err := c.startPerformanceChange(next, false, "Using the verified automatic clock limit"); err != nil {
			c.stop("Could not apply the automatic clock limit")
			return err
		}
	}
	if !enabled && c.approved != defaultTuning {
		// The default is a defined operating point, not a customer-entered profile.
		if err := c.startPerformanceChange(defaultTuning, false, "Restoring SPECTRE defaults"); err != nil {
			c.stop("Could not restore SPECTRE defaults")
			return err
		}
	}
	return nil
}
