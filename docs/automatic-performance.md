# Automatic performance

Customers have two operating modes: SPECTRE defaults or Automatic performance.
The dashboard and authenticated API do not accept arbitrary clock or voltage
values. Automatic changes retain internal verification and crash recovery;
there is no customer confirmation, countdown, or temporary tuning mode.

## Baseline and initial bounds

| Setting | Policy for original KD BOX / MCB_V5_4, stock 2.2.1, XBT |
| --- | --- |
| SPECTRE default | 740 MHz, 430 mV |
| Automatic performance on new installations | Disabled; normal automatic fan control remains active |
| Automatic clock range | 500 MHz to the per-unit qualified ceiling; initial ceiling 740 MHz, factory hard bound 850 MHz |
| Automatic core voltage | Fixed 430 mV |
| Automatic fan output | 100% commanded on both fans while mining |
| Default-mode cooling | Existing hottest-chip control, 30% fan minimum |
| Hottest-chip limit | Saved Cooling slider, 70–80°C, default 75°C |

The pre-install backup on the test unit contained an 850 MHz / 430 mV power
plan. A saved stock setting does not establish a vendor-qualified XBT operating
point. The 740 MHz / 430 mV SPECTRE default is the repeatedly observed XBT point
on this unit. New installations retain a 740 MHz automatic ceiling until factory
qualification records a unit-specific limit. The factory search is bounded at
850 MHz, the clock observed in that stock snapshot. That is a test bound, not a
manufacturer-declared silicon maximum. Automatic voltage changes are disabled;
430 mV is held constant throughout clock qualification and operation.

Temperature cannot establish electrical or PSU headroom. Clock and voltage
therefore have independent hard bounds, and the controller holds voltage
constant while changing clock. Goldshell does not provide an overclocking guide
and
[does not recommend overclocking](https://goldshellhelp.zendesk.com/hc/en-us/articles/10563344384793-Do-you-provide-any-tutorials-or-guides-on-how-to-overclock-the-miner-or-on-the-immersion-cooling-process).
This policy is independent software for repurposed hardware; it is not vendor
certification or a guarantee for other units. Measure input power and qualify
the actual board, PSU and cooling assembly before expanding the bounds or
making customer performance claims.

## Control policy

Automatic mode is opt-in and available only for XBT on the verified 2.2.1 base.
It seeks a stable operating point within the cooling budget and its clock and
voltage bounds. A unit at its clock ceiling may remain cooler than the limit.

- The 15-second exponential hottest-chip filter governs normal clock changes.
  Above limit +1.5°C for 30 seconds, or raw limit +5°C for nine seconds, clock
  reduces by 25 MHz. At 500 MHz, inability to meet the limit stops mining.
- After five minutes of stable operation, at least 1,000 additional nonces and
  a new accepted share, sustained cooling headroom below limit -2°C for two
  minutes permits a 10 MHz increase, capped at the unit-specific qualified or
  learned limit.
- Clock increases require five minutes of fresh, stable
  operation, at least 1,000 new nonces and a new accepted share before they are
  saved. Isolated invalid nonces are normal and do not stop mining. Within a
  five-minute observation window, at least five new errors above 0.1% of new
  nonces, or a batch of 20 new errors regardless of nonce volume, rejects an
  increase. At an established automatic point, the same budget reduces clock
  by 25 MHz (floor 500 MHz) and persists the lower ceiling. At the floor, errors
  alone never stop mining. Missing or invalid quality counters pause optimization
  and restore a pending increase; they do not create an error-triggered stop.
  Missing accepted work eventually restores the previous point automatically.
- Rejection restores the last verified point and persists a tighter
  clock ceiling in the private configuration. Rebooting does not cause
  the same rejected point to be retried. Thermal downshifts keep the verified
  voltage and do not wait for the optimization qualification window.

Clock-only changes use the pinned stock driver’s privileged loopback
`pgaset` clock setter, with clock and fixed-voltage readback. They retain a
recovery journal and do not require an engine restart. If the live write cannot
be verified, the full stock profile path re-establishes the bounded point.
Bootstrap, full-profile recovery and switching back to defaults can restart
mining and retain stock calibration and sensor initialization checks. Counter
baselines are captured before live changes, or after fresh temperatures return
from a full restart. A retained clock keeps its verified observation window,
allowing the next step without discarding the preceding five minutes of data.
The customer can turn the
toggle off during an adjustment; this cancels the adjustment and restores the
fixed default automatically. Safety stops are retained.

## Factory clock qualification

The [2026-11-11 live qualification](clock-characterization.md) retained 755 MHz
at 430 mV on the test unit after a twenty-minute zero-error run; 760 MHz was
rejected by the hardware-error check. This does not raise the initial ceiling
for other units.

The customer HTTP API cannot select a numeric clock or raise its ceiling.
Factory administration uses an owner-only `clock-limit-request.json` in
`/usr/config/kdbox-xbt`, containing only `maximum_clock`, with mode 0600. A
SIGHUP to the verified controller PID validates the request, preserves the
passwords and other settings, and saves the per-unit ceiling. It cannot exceed
850 MHz or alter voltage. Raising a ceiling does not immediately apply a clock:
the automatic controller still requires thermal headroom, accepted work and
bounded-error qualification at every step. This also permits 5 MHz refinement
between the last passing point and a rejected point. A final sustained run is
recorded separately from the search. Results apply to that unit and test period;
a finite zero-error run does not establish an absolute silicon limit.

## Protection and recovery

Raw temperature shutdowns run before the optimizer. The stock 2.2.1 limits remain
84°C chip average, 90°C hottest chip and 86°C board. Missing/invalid chip readings,
fan faults and watchdog failures retain their stops. Automatic mode requires
all 16 chip temperature readings, checks each fan at the full-output RPM floor,
and verifies the live clock and voltage. Out-of-policy runtime settings stop
mining, including changes made outside the new dashboard.

`pending.json` records the previous and proposed points before an API write.
An interrupted adjustment restores the previous point through the existing
preflight and controller recovery paths. `approved.json` changes only after
verification, a conservative clock downshift, or restoration of the fixed
default. A hard temperature or fan fault stops mining immediately and retains
the journal for recovery. It does not first attempt a live profile rollback.

The private configuration retains the mode and learned limits without changing
the dashboard or SSH passwords. Turning automatic mode off restores 740 MHz /
430 mV; restarting with automatic mode off also discards an old manual operating
point in favor of that defined default. Invalid nonces never create a safety
stop in either mode. Default mode does
not use quality counters to gate mining. Thermal, fan, telemetry, watchdog and
clock/voltage faults retain their safety stops. Historical qualification records
retain the stricter error policy used during those tests.
