package main

import (
	"errors"
	"fmt"
	"math"
	"testing"
	"time"
)

func TestFanStartupRecheck(t *testing.T) {
	for _, tc := range []struct {
		name        string
		startup     bool
		first, next int
		err         error
		want        bool
		reads       int
	}{
		{"healthy", true, 4700, 4700, nil, true, 1},
		{"spinup", true, 0, 4700, nil, true, 2},
		{"stalled", true, 0, 0, nil, false, 2},
		{"sensor-error", true, 4700, 4700, errors.New("sensor"), false, 2},
		{"runtime-fault", false, 0, 4700, nil, false, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reads, waits := 0, 0
			got := fanReady(func() (int, error) {
				reads++
				if reads == 1 {
					return tc.first, tc.err
				}
				return tc.next, tc.err
			}, 3000, tc.startup, func(d time.Duration) {
				waits++
				if d != 2*time.Second {
					t.Fatal(d)
				}
			})
			if got != tc.want || reads != tc.reads || waits != tc.reads-1 {
				t.Fatalf("ready=%v reads=%d waits=%d", got, reads, waits)
			}
		})
	}
}

func TestCoolingHoldsThroughTemperatureJitter(t *testing.T) {
	now := time.Now()
	f := automaticCooling{}
	for i := 0; i < 80; i++ {
		f.next(now.Add(time.Duration(i)*10*time.Second), 50, 60, 60, 70, 84, 90)
	}
	if f.percent != 30 {
		t.Fatal("minimum fan output", f.percent)
	}
	f = automaticCooling{percent: 55}
	for i := 0; i < 20; i++ {
		f.next(now.Add(time.Duration(i)*3*time.Second), 50, 64, 70, 70, 84, 90)
	}
	for i := 0; i < 100; i++ {
		peak := 70.0 + float64(i%3-1)
		got := f.next(now.Add(time.Duration(60+i*3)*time.Second), 50, 64, peak, 70, 84, 90)
		if got != 55 {
			t.Fatal("fan hunted inside the temperature band", got)
		}
	}
	for i := 0; i < 20; i++ {
		f.next(now.Add(time.Duration(360+i*3)*time.Second), 50, 65, 73, 70, 84, 90)
	}
	if f.percent <= 55 {
		t.Fatal("sustained heat did not increase cooling", f.percent)
	}
}

func TestBriefChipSpikesDoNotLatchBoost(t *testing.T) {
	now := time.Now()
	f := automaticCooling{percent: 85, updated: now, sampled: now, peak: 70, board: 50, average: 60, target: 70}
	for seconds := -21; seconds <= 0; seconds += 3 {
		f.predict(now.Add(time.Duration(seconds)*time.Second), [3]float64{50, 60, 70})
	}
	for _, sample := range []struct {
		seconds int
		peak    float64
	}{{3, 77}, {6, 77}, {9, 70}, {12, 70}, {15, 76}, {18, 70}} {
		got := f.next(now.Add(time.Duration(sample.seconds)*time.Second), 50, 60, sample.peak, 70, 84, 90)
		if got == 100 || !f.boostUntil.IsZero() {
			t.Fatal("brief noncritical spike latched full output", sample, got)
		}
	}
	for i := 30; i <= 39; i += 3 {
		f.next(now.Add(time.Duration(i)*time.Second), 50, 60, 77, 70, 84, 90)
	}
	if f.percent != 100 || f.boostUntil.IsZero() {
		t.Fatal("sustained hot chip failed to command full cooling")
	}
}

func TestCoolingReductionsRequireTrendHistoryAndSettling(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		peak     float64
		step     int
		interval int
	}{{60, 2, 6}, {64, 1, 12}, {67, 1, 30}} {
		f := automaticCooling{percent: 100, updated: now}
		for seconds := 0; seconds <= 33; seconds += 3 {
			if f.next(now.Add(time.Duration(seconds)*time.Second), 50, 60, tc.peak, 70, 84, 90) != 100 {
				t.Fatal("trend history or cooling dwell was bypassed")
			}
		}
		if got := f.next(now.Add(36*time.Second), 50, 60, tc.peak, 70, 84, 90); got != 100-tc.step {
			t.Fatal("wrong reduction", got)
		}
		for seconds := 39; seconds < 36+tc.interval; seconds += 3 {
			if got := f.next(now.Add(time.Duration(seconds)*time.Second), 50, 60, tc.peak, 70, 84, 90); got != 100-tc.step {
				t.Fatal("settling interval was bypassed", got)
			}
		}
		if got := f.next(now.Add(time.Duration(36+tc.interval)*time.Second), 50, 60, tc.peak, 70, 84, 90); got != 100-2*tc.step {
			t.Fatal("reduction did not continue", got)
		}
	}
}

func TestCoolingStopsStaleIncreasesAfterExcursionEnds(t *testing.T) {
	now := time.Now()
	f := automaticCooling{percent: 60}
	for seconds := 0; seconds <= 60; seconds += 3 {
		f.next(now.Add(time.Duration(seconds)*time.Second), 50, 66, 78, 80, 84, 90)
	}
	for seconds := 63; seconds <= 78; seconds += 3 {
		f.next(now.Add(time.Duration(seconds)*time.Second), 50, 66, 82, 80, 84, 90)
	}
	first := f.next(now.Add(81*time.Second), 50, 66, 76, 80, 84, 90)
	second := f.next(now.Add(84*time.Second), 50, 66, 78, 80, 84, 90)
	if f.projected[2] <= 80.5 || second != first || !f.boostUntil.IsZero() {
		t.Fatalf("stale forecast kept increasing fans: forecast %.1f, %d -> %d", f.projected[2], first, second)
	}
}

func TestCoolingSettlesForecastsButRespondsQuicklyToMeasuredHeat(t *testing.T) {
	now := time.Now()
	f := automaticCooling{percent: 60, updated: now, headroom: 6, headroomGoal: 6, headroomSeen: now, headroomAt: now}
	for seconds := -60; seconds <= 0; seconds += 3 {
		f.predict(now.Add(time.Duration(seconds)*time.Second), [3]float64{50, 66, 75})
	}
	for seconds := 3; seconds <= 12; seconds += 3 {
		if got := f.next(now.Add(time.Duration(seconds)*time.Second), 50, 66, 75, 80, 84, 90); got != 60 {
			t.Fatal("forecast corrections stacked before airflow could settle", got)
		}
	}
	if got := f.next(now.Add(15*time.Second), 50, 66, 75, 80, 84, 90); got > 62 || got <= 60 {
		t.Fatal("forecast did not make one bounded adjustment", got)
	}
	before := f.percent
	f.next(now.Add(18*time.Second), 50, 66, 82, 80, 84, 90)
	if got := f.next(now.Add(21*time.Second), 50, 66, 82, 80, 84, 90); got <= before {
		t.Fatal("confirmed heat waited for the forecast settling interval", got)
	}
}

// A delayed thermal plant catches the overshoot that fixed-temperature tests
// miss: airflow changes affect later samples, not the temperature at the write.
func TestCoolingSettlesWithThermalLagAndChangingAmbient(t *testing.T) {
	for _, target := range []float64{70, 80} {
		for _, lag := range []float64{30, 60, 90} {
			for _, spikeSeconds := range []int{0, 6, 12} {
				t.Run(fmt.Sprintf("target-%.0f-lag-%.0f-cached-%ds", target, lag, spikeSeconds), func(t *testing.T) {
					now := time.Now()
					f := automaticCooling{}
					chip := 40.0
					delayed := []int{100, 100, 100, 100}
					maxPeak, maxChip, sum := 0.0, 0.0, 0.0
					lo, hi, count, boosts := 100, 0, 0, 0
					loSettled, hiSettled := 100, 0
					settledMeasuredPeak := 0.0
					for seconds := 0; seconds <= 2700; seconds += 3 {
						ambient := target - 45
						if seconds >= 900 && seconds < 1800 {
							ambient += 4
						}
						equilibrium := ambient + 34*math.Sqrt(100/float64(delayed[0]))
						chip += (equilibrium - chip) * (1 - math.Exp(-3/lag))
						peak := math.Round(chip) + float64((seconds/3)%3-1)
						// Stock caches can repeat a short excursion across
						// polls; do not mistake that for sustained heating.
						steadyPhase := seconds%900 >= 400
						if spikeSeconds > 0 && steadyPhase && seconds%90 < spikeSeconds {
							peak += 4
						}
						percent := f.next(now.Add(time.Duration(seconds)*time.Second), 45, peak-10, peak, target, 84, 90)
						delayed = append(delayed[1:], percent)
						maxPeak = max(maxPeak, peak)
						maxChip = max(maxChip, chip)
						if !f.boostUntil.IsZero() {
							boosts++
						}
						if seconds >= 2400 {
							loSettled, hiSettled = min(loSettled, percent), max(hiSettled, percent)
							settledMeasuredPeak = max(settledMeasuredPeak, peak)
						}
						if seconds >= 2100 {
							lo, hi = min(lo, percent), max(hi, percent)
							sum += chip
							count++
						}
					}
					mean := sum / float64(count)
					t.Logf("measured peak %.1f, physical peak %.1f, settled mean %.1f, duty %d..%d, settled %d..%d, boost samples %d", maxPeak, maxChip, mean, lo, hi, loSettled, hiSettled, boosts)
					// Injected positive measurement excursions can exceed the
					// actual chip temperature and appropriately bias cooling
					// lower. Bound physical overshoot separately from that noise.
					measuredLimit := target + 3
					if spikeSeconds > 0 {
						measuredLimit = min(target+4, 84)
					}
					// The last five minutes start ten minutes after the ambient
					// step, allowing the slowest plant and reserve to settle.
					if maxChip > target+2 || maxPeak > measuredLimit || boosts != 0 || mean < target-7 || mean > target+1 || settledMeasuredPeak < target-3 || hiSettled-loSettled > 15 {
						t.Fatal("thermal lag or ambient step caused overshoot/hunting")
					}
				})
			}
		}
	}
}

func TestCriticalRawHeatBypassesSmoothingAndBoostHasHysteresis(t *testing.T) {
	now := time.Now()
	f := automaticCooling{percent: 85, updated: now, sampled: now, peak: 65, board: 50, average: 60, target: 70}
	if got := f.next(now.Add(time.Second), 50, 60, 74, 70, 84, 90); got != 85 || f.peak >= 66 {
		t.Fatal("brief noncritical reading bypassed smoothing", got, f.peak)
	}
	if got := f.next(now.Add(2*time.Second), 50, 60, 85, 70, 84, 90); got != 100 {
		t.Fatal("raw hot chip did not immediately command full output", got)
	}
	if got := f.next(now.Add(40*time.Second), 50, 65, 72, 70, 84, 90); got != 100 || f.boostUntil.IsZero() {
		t.Fatal("boost released above its hysteresis threshold")
	}
	f.next(now.Add(41*time.Second), 50, 65, 70, 70, 84, 90)
	f.next(now.Add(50*time.Second), 50, 65, 70, 70, 84, 90)
	if f.boostUntil.IsZero() {
		t.Fatal("boost released before the cool dwell")
	}
	f.next(now.Add(57*time.Second), 50, 65, 70, 70, 84, 90)
	if !f.boostUntil.IsZero() {
		t.Fatal("boost did not release after sustained cooling")
	}
	if got := f.next(now.Add(58*time.Second), 0, 65, 70, 70, 84, 90); got != 100 {
		t.Fatal("invalid sensor did not command full cooling")
	}
	f = automaticCooling{percent: 30}
	if got := f.next(now, 50, 70, 85, 80, 84, 90); got != 100 {
		t.Fatal("higher target weakened the raw safety boost")
	}
	for _, reading := range [][3]float64{{75, 65, 70}, {50, 80, 70}} {
		f = automaticCooling{percent: 30}
		if f.next(now, reading[0], reading[1], reading[2], 70, 84, 90) != 100 {
			t.Fatal("critical board/average temperature did not immediately boost")
		}
	}
}

func TestTargetChangeDiscardsOldReserveAndForecastWithoutDroppingFanOutput(t *testing.T) {
	now := time.Now()
	f := automaticCooling{percent: 65, target: 70, headroom: 5, headroomGoal: 5, headroomSeen: now, headroomAt: now}
	for second := -60; second <= 0; second += 3 {
		f.next(now.Add(time.Duration(second)*time.Second), 50, 60, 70, 70, 84, 90)
	}
	before := f.percent
	f.retarget(75)
	if f.percent != before || f.headroom != 0 || f.headroomGoal != 0 || len(f.raw) != 0 || len(f.history) != 0 || f.direction != 0 {
		t.Fatal("old operating point survived", f)
	}
	for second := 3; second <= 60; second += 3 {
		f.next(now.Add(time.Duration(second)*time.Second), 50, 60, 70, 75, 84, 90)
	}
	if f.percent >= before || f.headroom != 0 {
		t.Fatal("fans remained stuck at the old target", before, f.percent, f.headroom)
	}
	// Retrying an unchanged target must not discard new, valid observations.
	n := len(f.raw)
	f.retarget(75)
	if len(f.raw) != n {
		t.Fatal("no-op target erased history")
	}
	f.boostUntil = now.Add(2 * time.Minute)
	boost := f.boostUntil
	f.retarget(80)
	if f.boostUntil != boost || f.next(now.Add(63*time.Second), 50, 60, 85, 80, 84, 90) != 100 {
		t.Fatal("target edit bypassed raw protection")
	}
}
func TestCoolingObservationStorageRemainsBoundedAcrossMonthsAndClockChanges(t *testing.T) {
	now := time.Now()
	f := automaticCooling{percent: 65}
	// Nine samples a minute for 90 days, plus an abnormal same-timestamp burst.
	for i := 0; i < 90*24*60*9; i++ {
		at := now.Add(time.Duration(i) * 7 * time.Second)
		f.next(at, 50, 60, 70, 75, 84, 90)
		if len(f.raw) > 11 || cap(f.raw) > 32 || len(f.history) > 32 || cap(f.history) > 64 {
			t.Fatal("cooling storage grew", len(f.raw), len(f.history))
		}
	}
	for i := 0; i < 1000; i++ {
		f.predict(now, [3]float64{50, 60, 70})
	}
	if len(f.history) > 32 || len(f.raw) > 11 {
		t.Fatal("clock rollback or rapid samples grew storage")
	}
}

func TestCoolingTargetStepsSettleAtNewOperatingPoint(t *testing.T) {
	for _, lag := range []float64{30, 60, 90} {
		now := time.Now()
		f := automaticCooling{}
		chip := 40.0
		delayed := [4]int{100, 100, 100, 100}
		var sums [3]float64
		var counts [3]int
		for second := 0; second < 10800; second += 3 {
			phase := second / 3600
			target := 70.0
			if phase == 1 {
				target = 75
			}
			equilibrium := 25 + 34*math.Sqrt(100/float64(delayed[0]))
			chip += (equilibrium - chip) * (1 - math.Exp(-3/lag))
			peak := math.Round(chip) + float64((second/3)%3-1)
			duty := f.next(now.Add(time.Duration(second)*time.Second), 45, peak-10, peak, target, 84, 90)
			copy(delayed[:], delayed[1:])
			delayed[3] = duty
			if peak >= 85 || !f.boostUntil.IsZero() {
				t.Fatal("target edit caused overheating/boost", lag, second, peak)
			}
			if second%3600 >= 3300 {
				sums[phase] += chip
				counts[phase]++
			}
		}
		means := [3]float64{}
		for i := range means {
			means[i] = sums[i] / float64(counts[i])
		}
		if means[1] < means[0]+3 || math.Abs(means[2]-means[0]) > 2 {
			t.Fatal("target remained stuck", lag, means)
		}
	}
}

func TestLearnedSpikeReserveIsHalfThePreviousAllowance(t *testing.T) {
	now := time.Now()
	for _, excess := range []float64{0, 1, 2, 4, 6, 8, 12} {
		f := automaticCooling{raw: make([]coolingReading, 11), headroomAt: now, projected: [3]float64{50, 60, 70}}
		peak := 70 + excess
		f.reserveHeadroom(now, peak, 70, 90, true)
		want := 0.5 * max(0, min(6, excess-1))
		if f.headroomGoal != want || f.headroomGoal > 3 {
			t.Fatal("spike reserve was not halved", excess, f.headroomGoal, want)
		}
	}
	f := automaticCooling{percent: 30}
	if duty := f.next(now, 50, 65, 85, 75, 84, 90); duty != 100 {
		t.Fatal("smaller normal reserve weakened raw full-fan boost", duty)
	}
}
