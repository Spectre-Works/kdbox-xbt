package main

import (
	"errors"
	"fmt"
)

type Tuning struct {
	Millivolts int `json:"millivolts"`
	Clock      int `json:"clock"`
}

// SPECTRE's observed XBT baseline, not a vendor-certified XBT power plan.
const maximumAutomaticClock = 850

var defaultTuning = Tuning{Millivolts: 430, Clock: 740}

func (t Tuning) automaticBounds() error {
	if t.Clock < 500 || t.Clock > maximumAutomaticClock || t.Clock%5 != 0 ||
		t.Millivolts != defaultTuning.Millivolts {
		return errors.New("automatic performance is limited to the qualified clock range, at 430 mV")
	}
	return nil
}

func (t Tuning) Validate(previous Tuning) error {
	if t.Millivolts < 400 || t.Millivolts > 430 || t.Millivolts%5 != 0 {
		return errors.New("voltage must be 400–430 mV in 5 mV steps; above-stock voltage is disabled")
	}
	if t.Millivolts-previous.Millivolts > 5 || previous.Millivolts-t.Millivolts > 5 {
		return errors.New("change voltage by at most 5 mV per adjustment")
	}
	if t.Clock < 500 || t.Clock > maximumAutomaticClock || t.Clock%5 != 0 {
		return errors.New("clock must be 500–850 MHz in 5 MHz steps")
	}
	if t.Clock > previous.Clock+25 {
		return errors.New("increase by at most 25 MHz per adjustment")
	}
	return nil
}
func (t Tuning) Powerplan() string {
	return fmt.Sprintf("%d MHz %.3f V 100 RPM 100 RPM", t.Clock, float64(t.Millivolts)/1000)
}

// Counter deltas measure invalid nonces, not errors divided by pool shares.
func errorRate(oldHW, oldNonces, hw, nonces float64) (float64, bool) {
	if hw < oldHW || nonces <= oldNonces {
		return 0, false
	}
	return 100 * (hw - oldHW) / (nonces - oldNonces), true
}
