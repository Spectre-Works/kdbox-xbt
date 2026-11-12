# Cooling overshoot investigation — 2026-11-11

The original KD BOX / MCB_V5_4 at `192.168.86.181`, on pinned stock 2.2.1,
was tested at **740 MHz / 430 mV**, Automatic performance **off**, and a saved
**80°C hottest-chip limit**. The normal fan floor remained **30%**.

## Failure reproduced

Before the fix, 182 samples over 555 seconds reproduced
fan reductions during warming. At about 91 seconds, the hottest chip was 79°C
with fans at 37%. At about 122 seconds, it was 84°C with fans at 34%, but the
15-second filtered reading was only 80.9°C. Increases waited for that filtered
reading to exceed 81.5°C. The old reduction hysteresis continued removing
cooling while the reported temperature rose, and the normal increase rate
could not recover the lost airflow before the 85°C full-fan protection.

The observed peak reached **86°C**, fan readback reached **33%**, and
**2 full-fan protection episodes** were captured. The first boost was observed
at 166 seconds. Stock PWM readback can show 99–100% for a full-output
command. This was a late normal-control response; the protective boost remained
necessary once its raw threshold was reached.

## Corrected control

- Keep the normal 30% minimum and saved 70–80°C limit range.
- Use an eleven-reading median for normal control, spanning 30 seconds at the
  usual three-second polling interval. Stock telemetry can repeat a brief
  excursion across multiple polls; a three-reading median was insufficient.
- Estimate a bounded trend over 30 seconds. Weight a noisy regression by its
  fit quality, and project warming 30 seconds ahead. Falling trends never
  justify extra fan reductions.
- Learn bounded headroom for positive hottest-chip excursions beyond normal
  filter lag. Blend this reserve in gradually, cap it at 6°C and decay it
  slowly after quiet operation. The limit governs the upper range of
  readings, so the usual or median reading can be several degrees lower.
- Begin normal increases above projected limit +0.5°C, one or two percentage
  points per fifteen seconds with time for airflow to settle. Confirmed hot
  readings can increase four to ten points per three seconds; near-limit raw
  readings bypass confirmation. Two fresh cooler readings stop
  stale forecast-driven increases; they never accelerate reductions.
- Require trend history and a 20-second cool dwell before reducing output.
  Reduce two points per six seconds with more than 8°C of projected headroom,
  one per twelve seconds with 4–8°C, and one per thirty seconds nearer the
  limit. Near-limit warming cancels reductions and restarts the dwell.
- Keep all raw thermal protections independent of normal filtering. An 85°C
  raw hottest-chip reading still commands full fans immediately. Stock 2.2.1
  shutdowns remain 90°C hottest chip, 84°C chip average and 86°C board. Fan
  checks, watchdogs and invalid-sensor handling remain active.

Live measurements also exposed repeated stock excursions and stale positive
forecasts after cooling. The median, fit weighting and two-reading cooling
confirmation address those effects without weakening raw protection. The
[architecture](architecture.md) describes the current policy. The Live
verification section below refers to the final build.

## Separate hardware-quality stop

An earlier run of the headroom-aware build recorded one native invalid nonce
out of 11,679 reported nonces at 740 MHz / 430 mV, with a 72°C hottest chip,
65°C chip average, 52.3°C board and zero pool rejections. The unchanged quality
guard stopped mining. The native log and stop snapshot were retained. The
cause of that individual invalid nonce is not established by the temperature
readings. Additional supervised testing retained the same clock, voltage and
quality guard. The final build's verification results are below. This testing
does not erase the earlier event or establish long-term error-free hardware
operation.

## Live verification

The final run lasted **1297 seconds**, including stock fan
calibration and **at least twenty minutes after fresh chip readings**. It
captured **801 complete 16-chip reports**, with a highest
observed raw chip reading of **80°C**. **No emergency boost was
needed**, and there were **zero hardware errors**.

During the final ten-minute window (599 seconds,
400 samples):

| Measurement | Observed result |
| --- | --- |
| Mean hottest-chip reading | 74.1°C |
| Hottest-chip range | 71–80°C |
| Fan duty range | 62–74% |
| Full-fan boost samples | 0 |
| Hardware errors | 0 |

The learned hottest-chip headroom reached **6.0°C**. Typical readings
therefore remain below 80°C so that the upper excursions can stay near the
selected limit. The 80°C configuration value was not changed.

The miner reported **198 accepted shares** at the end;
DATUM independently showed **197**
for its current client session. Asynchronous polls can differ briefly. All
original password files, the 80°C limit, default operating mode and the saved
755 MHz automatic ceiling matched their pre-test checks. No firmware flashing
or reboot was used. Raw samples, all-chip CSVs, DATUM observations and rollback
snapshots remain private on the test host.

KD BOX Control is again the larger header label, with SPECTRE WORKS secondary.
The company emblem remains. The current build uses the SPECTRE WORKS website
favicons; the earlier favicon is no longer shipped. Desktop/mobile checks passed
on both web ports in light and dark themes. The build, Go tests/race/vet,
59 Python tests and JavaScript checks passed. Eighteen delayed thermal-model
cases cover 70/80°C limits, 30/60/90-second thermal lags, ambient changes and
repeated cached sensor excursions; immediate raw protection has separate tests.

Controller SHA-256:

```text
bc2260e734e7d5c3159d4785d02157c8f5019163731811c34d0bd235c12aff51
```

These observations apply to this unit and test period. Temperature noise and
changed ambient conditions remain possible; the full-fan and shutdown
protections are intentionally retained.

## Invalid-nonce policy update

The isolated invalid nonce and resulting stop described above are historical
evidence of the former policy. Invalid nonces no longer trigger a safety stop.
Default-mode hashing ignores quality counters as a control input; only automatic
performance uses a bounded error budget to reduce clock or reject an increase.
Thermal, fan and telemetry protection is unchanged.

## Changing the limit

A manual limit change now discards reserve, median/trend history and dwell from
the previous operating point. It preserves current duty, physical filtering and
any active boost; raw fan/temperature/watchdog protection does not change. The
same-limit retry leaves valid observations intact. Automatic performance resets
only limit-relative thermal dwell, preserving its error counters and qualified
clock envelope. See the [endurance review](dashboard-endurance.md) for transition
models, bounded storage and live verification.

Automatic performance always commands full fan output and can run below the
selected limit once its qualified clock ceiling is reached. Raising the limit
does not authorize a higher clock ceiling or command extra heat.

## Current installation defaults

New installations default to a **75°C hottest-chip limit** with **Automatic
performance disabled**. Normal automatic fan control remains active with the
30% minimum; the fixed SPECTRE operating point is 740 MHz / 430 mV. The limit
slider still accepts 70–80°C, and automatic performance remains an optional
owner choice. Existing saved choices are preserved by a software update; the
operator can explicitly change them through the dashboard. Earlier measurements
in this document retain the limits and modes used during those tests.

## Reduced spike allowance and mining-session display (2026-11-12)

Normal fan control now applies **50%** of the previous learned spike allowance:
`0.5 × clamp(raw peak − median − trend lag − 1°C, 0, 6°C)`. Its maximum is
therefore **3°C**, rather than 6°C. The median, trend forecast, settling/hysteresis,
30% minimum, raw 85°C full-fan boost, 90°C peak / 84°C average / 86°C board stops,
fan checks and watchdog remain unchanged. Earlier measurements above describe
the original larger reserve; they are not measurements of this revision.

Overview adds a **Mining session** duration in Mining configuration. The
controller starts the interval on positive hashrate with fresh, ready telemetry
and resets it on zero hashing, a stop, engine startup or stale telemetry. Short
telemetry delays within the existing 15-second freshness window retain the
interval, matching the active-state display. Browser refreshes and laptop network
interruptions do not reset a continuously active miner. Status updates carry the
duration; no per-second browser timer or persistent write was added.

The dashboard labels this setting **Hottest-chip limit**. It is the operating
limit used by cooling and automatic performance; brief excursions can still
occur. It is separate from the unchanged critical thermal shutdown limits.
The persisted `chip_target` field and API `target` key retain their names so
existing saved settings and installers continue to work.

### Verification of the half-reserve build

The 2026-11-12 live run on the original KD BOX at `192.168.86.181` lasted
400.5 seconds, including more than five minutes after startup.
It captured 146 complete 16-chip reports with a highest
reading of 73°C. The learned reserve stayed within 3°C; final
fan duty was 82% / 82%. No normal-operation
full-fan boost was observed. The miner and DATUM each reported
15 accepted shares at completion, including new work after
fresh telemetry became available.

Both dashboard ports verified the Mining session row, persistence across
browser reloads, gray Manage mining button/navigation, Hottest-chip limit label,
SPECTRE WORKS link and mobile layout. The controller's final session duration was
301 seconds. Password/configuration/approved-setting
hashes matched; 75°C, 740 MHz / 430 mV, Automatic performance off, the 30% normal
fan floor and the unit's qualified ceiling were preserved. These observations
apply to this unit and period, not a guarantee for every miner or environment.
