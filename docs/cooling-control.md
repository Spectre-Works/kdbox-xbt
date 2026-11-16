# Automatic cooling control

## Corrected control

- Keep the normal 30% minimum and saved 70–80°C limit range.
- Use an eleven-reading median for normal control, spanning 30 seconds at the
  usual three-second polling interval. Stock telemetry can repeat a brief
  excursion across multiple polls; a three-reading median was insufficient.
- Estimate a bounded trend over 30 seconds. Weight a noisy regression by its
  fit quality, and project warming 30 seconds ahead. Falling trends never
  justify extra fan reductions.
- Learn bounded headroom for positive hottest-chip excursions beyond normal
  filter lag. Blend this reserve in gradually, apply half of the learned allowance, capped at 3°C and decay it
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


## Changing the limit

A manual limit change now discards reserve, median/trend history and dwell from
the previous operating point. It preserves current duty, physical filtering and
any active boost; raw fan/temperature/watchdog protection does not change. The
same-limit retry leaves valid observations intact. Automatic performance resets
only limit-relative thermal dwell, preserving its error counters and qualified
clock envelope. See the [endurance review](dashboard-endurance.md) for transition
models and bounded storage.

Automatic performance always commands full fan output and can run below the
selected limit once its qualified clock ceiling is reached. Raising the limit
does not authorize a higher clock ceiling or command extra heat.

## Current installation defaults

New installations default to a **75°C hottest-chip limit** with **Automatic
performance disabled**. Normal automatic fan control remains active with the
30% minimum; the fixed SPECTRE operating point is 740 MHz / 430 mV. The limit
slider still accepts 70–80°C, and automatic performance remains an optional
owner choice. Existing saved choices are preserved by a software update; the
operator can explicitly change them through the dashboard.

Normal fan control now applies **50%** of the previous learned spike allowance:
`0.5 × clamp(raw peak − median − trend lag − 1°C, 0, 6°C)`. Its maximum is
therefore **3°C**, rather than 6°C. The median, trend forecast, settling/hysteresis,
30% minimum, raw 85°C full-fan boost, 90°C peak / 84°C average / 86°C board stops,
fan checks and watchdog remain unchanged.

Overview shows a **Mining session** duration in Mining configuration. The
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
