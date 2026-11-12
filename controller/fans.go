package main

import (
	"fmt"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	boardTemperatureTarget = 70.0
	chipTemperatureTarget  = 75
	minimumFanPercent      = 30
)

type automaticCooling struct {
	percent                     int
	updated, sampled, coolSince time.Time
	boostUntil, releaseSince    time.Time
	hotSince                    time.Time
	peak, board, average        float64
	direction                   int
	target                      float64
	raw, history                []coolingReading
	trend, projected            [3]float64
	headroom, headroomGoal      float64
	headroomSeen, headroomAt    time.Time
	headroomDecay               time.Time
}

type coolingReading struct {
	at     time.Time
	values [3]float64 // board, chip average, hottest chip
}

// Stock telemetry can repeat a brief excursion for multiple polls. A eleven-
// reading median (30 seconds at the normal cadence) rejects those excursions
// before estimating the thirty-second trend. Raw safety checks bypass both.
// A target edit starts a new operating-point observation. Keep actual output,
// filtered physical temperatures and any active safety boost; discard learned
// target-relative reserve, forecasts and dwell accumulated at the old target.
func (f *automaticCooling) retarget(target float64) {
	if f.target == target {
		return
	}
	f.target = target
	f.direction = 0
	f.coolSince, f.hotSince = time.Time{}, time.Time{}
	f.raw, f.history = nil, nil
	f.trend, f.projected = [3]float64{}, [3]float64{}
	f.headroom, f.headroomGoal = 0, 0
	f.headroomSeen, f.headroomAt, f.headroomDecay = time.Time{}, time.Time{}, time.Time{}
}

func appendCoolingReading(readings []coolingReading, reading coolingReading, maximum int) []coolingReading {
	if len(readings) == maximum {
		copy(readings, readings[1:])
		readings[len(readings)-1] = reading
		return readings
	}
	return append(readings, reading)
}

func (f *automaticCooling) predict(now time.Time, values [3]float64) (trusted [3]float64, ready bool) {
	if n := len(f.raw); n > 0 && now.Sub(f.raw[n-1].at) > 15*time.Second {
		f.raw, f.history = nil, nil
		f.coolSince = time.Time{}
	}
	f.raw = appendCoolingReading(f.raw, coolingReading{now, values}, 11)
	trusted = values
	for i := range trusted {
		var ordered [11]float64
		for j, r := range f.raw {
			ordered[j] = r.values[i]
		}
		window := ordered[:len(f.raw)]
		slices.Sort(window)
		trusted[i] = (window[(len(window)-1)/2] + window[len(window)/2]) / 2
	}
	f.history = appendCoolingReading(f.history, coolingReading{now, trusted}, 32)
	for len(f.history) > 1 && now.Sub(f.history[0].at) > 30*time.Second {
		copy(f.history, f.history[1:])
		f.history = f.history[:len(f.history)-1]
	}
	f.trend = [3]float64{}
	ready = len(f.history) >= 3 && now.Sub(f.history[0].at) >= 15*time.Second
	if ready {
		var sx, sxx float64
		var sy, syy, sxy [3]float64
		for _, reading := range f.history {
			x := reading.at.Sub(f.history[0].at).Seconds()
			sx += x
			sxx += x * x
			for i, y := range reading.values {
				sy[i] += y
				syy[i] += y * y
				sxy[i] += x * y
			}
		}
		n := float64(len(f.history))
		if denominator := n*sxx - sx*sx; denominator > 0 {
			for i := range f.trend {
				covariance := n*sxy[i] - sx*sy[i]
				if variance := n*syy[i] - sy[i]*sy[i]; variance > 0 {
					// A noisy, poorly fitting slope is weak evidence of warming.
					fit := max(0, min(1, covariance*covariance/(denominator*variance)))
					f.trend[i] = max(-0.25, min(0.25, covariance/denominator)) * fit
				}
			}
		}
	}
	for i := range trusted {
		// Predict warming, never use a falling trend to justify extra reductions.
		f.projected[i] = trusted[i] + max(0, f.trend[i])*30
	}
	return trusted, ready
}

// The target applies to the upper range of hottest-chip readings, not just
// their median. Learn positive excursions beyond the median's normal lag.
// Blend the reserve to avoid turning one noncritical spike into a fan jump.
func (f *automaticCooling) reserveHeadroom(now time.Time, peak, trustedPeak, peakLimit float64, ready bool) {
	if ready && len(f.raw) == 11 {
		// Use half the observed spike reserve (maximum 3 C). Raw heat
		// boosts and shutdown checks remain independent of this allowance.
		wanted := 0.5 * max(0, min(6, peak-trustedPeak-15*max(0, f.trend[2])-1))
		if wanted >= f.headroomGoal-0.25 {
			f.headroomSeen = now
		}
		if wanted > f.headroomGoal {
			f.headroomGoal = wanted
		}
	}
	if f.headroomGoal > 0 && now.Sub(f.headroomSeen) > 5*time.Minute && now.Sub(f.headroomDecay) >= time.Minute {
		f.headroomGoal = max(0, f.headroomGoal-0.25)
		f.headroomDecay = now
	}
	seconds := max(0, min(15, now.Sub(f.headroomAt).Seconds()))
	if f.headroomAt.IsZero() {
		seconds = 0
	}
	if peak >= peakLimit-7 {
		f.headroom = max(f.headroom, f.headroomGoal)
	} else if f.headroom < f.headroomGoal {
		f.headroom = min(f.headroomGoal, f.headroom+0.03*seconds)
	} else {
		f.headroom = max(f.headroomGoal, f.headroom-0.005*seconds)
	}
	f.headroomAt = now
	f.projected[2] += f.headroom
}

func (f *automaticCooling) next(now time.Time, board, average, peak, target, averageLimit, peakLimit float64) int {
	if f.percent < minimumFanPercent {
		f.percent = 100
	}
	if math.IsNaN(board+average+peak) || math.IsInf(board+average+peak, 0) ||
		board <= 0 || average <= 0 || peak <= 0 {
		*f = automaticCooling{percent: 100}
		return f.percent
	}
	// Only real target edits discard a previous operating point.
	if f.target != 0 && f.target != target {
		f.retarget(target)
	} else {
		f.target = target
	}
	if f.sampled.IsZero() {
		f.peak, f.board, f.average = peak, board, average
	} else {
		alpha := 1 - math.Exp(-max(0, now.Sub(f.sampled).Seconds())/15)
		f.peak += alpha * (peak - f.peak)
		f.board += alpha * (board - f.board)
		f.average += alpha * (average - f.average)
	}
	f.sampled = now
	trusted, trendReady := f.predict(now, [3]float64{board, average, peak})
	f.reserveHeadroom(now, peak, trusted[2], peakLimit, trendReady)
	// A brief, noncritical chip excursion must not latch full output. Genuine
	// sustained heat still gets an early boost; readings near the hard limits
	// bypass this confirmation and all normal filtering/rate limits.
	if peak >= target+5 {
		if f.hotSince.IsZero() {
			f.hotSince = now
		}
	} else {
		f.hotSince = time.Time{}
	}
	sustainedHeat := !f.hotSince.IsZero() && now.Sub(f.hotSince) >= 9*time.Second
	if board >= boardTemperatureTarget+5 || average >= averageLimit-4 || peak >= peakLimit-5 || sustainedHeat || f.peak >= target+5 {
		f.boostUntil = now.Add(30 * time.Second)
		f.releaseSince = time.Time{}
	}
	if !f.boostUntil.IsZero() {
		f.percent = 100
		f.updated = now
		f.direction = 0
		f.coolSince = time.Time{}
		if peak > target+1 || board > boardTemperatureTarget+1 || average > averageLimit-8 {
			f.releaseSince = time.Time{}
		} else if f.releaseSince.IsZero() {
			f.releaseSince = now
		}
		if now.Before(f.boostUntil) || f.releaseSince.IsZero() || now.Sub(f.releaseSince) < 15*time.Second {
			return f.percent
		}
		f.boostUntil = time.Time{}
		f.releaseSince = time.Time{}
		return f.percent
	}
	controlError := max(f.projected[0]-boardTemperatureTarget, f.projected[1]-(averageLimit-8), f.projected[2]-target)
	increaseProjected := f.projected
	if n := len(f.raw); n >= 2 {
		for i := range trusted {
			latest := max(f.raw[n-1].values[i], f.raw[n-2].values[i])
			if latest < trusted[i]-1.5 {
				// A median/trend can remain hot after a real excursion ends.
				// Two cooler readings stop stale increases, but never permit
				// a faster reduction or bypass any raw protection.
				reserve := 0.0
				if i == 2 {
					reserve = f.headroom
				}
				increaseProjected[i] = min(increaseProjected[i], latest+reserve)
			}
		}
	}
	increaseError := max(increaseProjected[0]-boardTemperatureTarget, increaseProjected[1]-(averageLimit-8), increaseProjected[2]-target)
	urgentError := 0.0
	if n := len(f.raw); n >= 3 {
		limits := [3]float64{boardTemperatureTarget, averageLimit - 8, target}
		for i, limit := range limits {
			a, b, c := f.raw[n-3].values[i], f.raw[n-2].values[i], f.raw[n-1].values[i]
			median := a + b + c - min(a, b, c) - max(a, b, c)
			if c > limit+0.5 && median > limit+0.5 {
				urgentError = max(urgentError, min(c, median)-limit)
			}
		}
	}
	nearLimit := peak >= peakLimit-7 || board >= boardTemperatureTarget+3 || average >= averageLimit-6
	if nearLimit {
		urgentError = max(urgentError, 4)
	}
	if increaseError > 0.5 || urgentError > 0.5 {
		f.direction = 1
	} else if controlError < -2 {
		f.direction = -1
	} else if (f.direction > 0 && increaseError <= 0) || (f.direction < 0 && controlError >= -1) {
		f.direction = 0
	}
	// Near equilibrium, observe the response to the previous reduction before
	// taking more airflow away. Startup can still descend quickly with ample
	// predicted headroom, but rising temperatures cancel near-target reductions.
	warming := max(f.trend[0], f.trend[1], f.trend[2]) > 0.02
	if f.direction < 0 && (!trendReady || (controlError > -4 && warming)) {
		f.direction = 0
	}
	if f.direction >= 0 {
		f.coolSince = time.Time{}
	} else if f.coolSince.IsZero() {
		f.coolSince = now
	}
	if f.direction > 0 {
		// Forecast-only corrections must settle too: otherwise a lead term
		// repeatedly integrates the same predicted heat into excessive duty.
		interval := 15 * time.Second
		step := min(2, max(1, int(math.Ceil(increaseError/2))))
		if urgentError > 0.5 {
			interval = 3 * time.Second
			step = min(10, max(4, int(math.Ceil(2*urgentError))))
		}
		if now.Sub(f.updated) >= interval && (increaseError > 0.5 || urgentError > 0.5) {
			f.percent += step
			f.updated = now
		}
	} else if f.direction < 0 && now.Sub(f.coolSince) >= 20*time.Second {
		step := 1
		interval := 30 * time.Second
		if controlError < -8 {
			step = 2
			interval = 6 * time.Second
		} else if controlError < -4 {
			interval = 12 * time.Second
		}
		if now.Sub(f.updated) >= interval {
			f.percent -= step
			f.updated = now
		}
	}
	f.percent = max(minimumFanPercent, min(100, f.percent))
	return f.percent
}

func readFanPercent(index int) (int, error) {
	data, err := os.ReadFile(fmt.Sprintf("/sys/class/fans/fan%d/lvl", index))
	if err != nil {
		return 0, err
	}
	percent, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || percent < 0 || percent > 100 {
		return 0, fmt.Errorf("invalid fan duty")
	}
	return percent, nil
}

func writeFanPercent(index, percent int) error {
	path := fmt.Sprintf("/sys/class/fans/fan%d/lvl", index)
	// Rewriting a settled duty makes the asynchronous driver briefly expose
	// the command again before rounding it. Leave normal output steady;
	// full-speed protection always writes through immediately.
	if percent < 100 {
		current, err := readFanPercent(index)
		if err != nil {
			return err
		}
		if current == percent {
			return nil
		}
	}
	// The driver's queued PWM update can round down after the initial readback.
	command := min(100, percent+1)
	return os.WriteFile(path, []byte(strconv.Itoa(command)), 0600)
}

func fanPercentages() [2]*int {
	var readings [2]*int
	for index := range readings {
		if percent, err := readFanPercent(index); err == nil {
			readings[index] = &percent
		}
	}
	return readings
}

func readFanRPM(index int) (int, error) {
	data, err := os.ReadFile(fmt.Sprintf("/sys/class/fans/fan%d/rpm", index))
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

func fanReadings() [2]*int {
	var readings [2]*int
	for index := range readings {
		if rpm, err := readFanRPM(index); err == nil && rpm >= 0 {
			readings[index] = &rpm
		}
	}
	return readings
}

func (c *Controller) checkFans(startup bool, percent int) bool {
	minimumRPM := min(3000, max(1500, 3000*percent/100))
	for i := 0; i < 2; i++ {
		if !fanReady(func() (int, error) { return readFanRPM(i) }, minimumRPM, startup, time.Sleep) {
			c.stop(fmt.Sprintf("Fan %d below %d RPM at %d%% output; mining stopped", i+1, minimumRPM, percent))
			return false
		}
	}
	return true
}

// Startup and an increase in commanded output can precede the RPM response.
// Allow one two-second recheck while accelerating. Persistent faults still stop.
func fanReady(read func() (int, error), minimumRPM int, startup bool, wait func(time.Duration)) bool {
	rpm, err := read()
	if err == nil && rpm >= minimumRPM {
		return true
	}
	if !startup {
		return false
	}
	wait(2 * time.Second)
	rpm, err = read()
	return err == nil && rpm >= minimumRPM
}
