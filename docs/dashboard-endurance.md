# Dashboard and controller endurance review

Reviewed 2026-11-11 for software 1.0. These checks establish explicit resource
bounds and observed behavior. They are not proof of uninterrupted operation for
months on every miner, stock firmware, browser or network.

## Cooling limit changes

Changing the hottest-chip limit now starts a new observation of the operating
point. The controller discards the old learned excursion reserve, raw median /
trend window, forecast and limit-relative dwell. Otherwise a learned 5°C reserve
at 70°C could still constrain fan reductions after the owner selected 75°C.
Repeating the same limit does not clear newly collected data.

Actual fan duty, filtered physical temperatures, an active thermal boost, the
safety-stop record, qualified clock ceiling and automatic error qualification
remain intact. Lowering the limit still gets the normal fast response to real
heat. Unfiltered temperature limits and fan/watchdog protection are unchanged.

A temperature limit is an **upper operating objective**, not a request to heat
the chips. Automatic performance commands both fans to 100% and cannot exceed
its qualified clock ceiling. At that ceiling, temperatures can remain below the
selected limit. Normal fan control also retains board/average temperature
constraints and the 30% minimum, so it cannot always hold an exact chip reading.

## Findings and resulting bounds

| Area | Review and changes |
| --- | --- |
| Browser polling | Three registered periodic tasks; one pending status, pool and history request each. Hidden documents and cached/hidden pages stop periodic tasks and cancel read requests. Showing the page restarts the existing tasks without registering duplicates. |
| Browser requests | Eight-second deadline, with the timeout cleared on completion. Superseded diagnostic/private-page reads are aborted. Writes are not cancelled or automatically repeated by the background-read logic. |
| Chart | At most 1,440 samples retained for 24 hours. SVG children are replaced, never appended as an accumulating series; an empty chart clears old children. One temporary-range timer, replaced on each selection. |
| Pools | At most ten displayed entries. Changes to share counters alone no longer recreate controls. Actions validate the pool snapshot actually displayed, including when focus temporarily prevents a redraw. |
| Reports and private views | One report body, capped at 64 KiB, and at most 16 chip cards. Replacing a report releases the previous DOM. Locking clears private views and cancels their pending reads. Download object URLs are revoked. Wi-Fi display is capped at 128 networks. |
| Notifications and handlers | One reusable notification element with two replaceable timers. Navigation uses handlers registered once; element replacement does not register document/window listeners repeatedly. Theme persistence stores one small value. |
| Controller history | Hard cap of 1,440 samples plus the 24-hour cutoff, even with irregular timestamps. Loading reads at most 128 KiB plus an overflow byte. Pruning reuses storage; a read without expiration no longer copies the complete internal buffer. Snapshots remain independent for concurrent serialization. |
| Cooling history | Eleven raw readings and at most 32 trend observations, even after a clock rollback or an abnormal burst of calls. Bounded buffers reuse storage instead of repeatedly slicing away their capacity. |
| Diagnostics | One stock report at a time; concurrent requests receive a retry response. Native report reads are capped at 1 MiB and displayed tails at 64 KiB. The stock mining log is read directly from its tail, including when its file grows well beyond the JSON read limit. Tail strings are cloned so a small displayed tail cannot retain a large source string. |
| API concurrency | At most 32 accepted HTTP connections, with header/read/write/idle timeouts of 5/10/15/30 seconds. Native responses are capped at 1 MiB. Client disconnects cancel read-only native requests and diagnostic work. |
| Safety scheduling | Status serialization, pool queries, private settings reads and diagnostics do not hold the safety-state mutex while waiting for a browser or native read. State changes remain serialized. The collector and safety loop each have one worker, not a growing queue of timer goroutines. |
| Sessions | Existing cap of 100 sessions, expiration after 30 minutes, and expiration cleanup on login. A new session does not create a timer or goroutine. |
| Logs | Controller and preflight logs each have a 256 KiB current file and one 256 KiB previous tail. Oversized legacy files and oversized single messages are bounded. Rotation preserves the inode used by the supervisor's stderr descriptor. |
| Liveness | Heartbeat files are replaced atomically, without a transient empty file. The existing independent watchdog deadlines and thermal/fan safeguards remain active. |

On the live unit, `/tmp` is tmpfs. Controller history, logs and heartbeat updates
therefore use RAM-backed storage rather than a periodic write load on firmware
flash. Controller configuration is written for explicit changes or recovery,
not on every browser telemetry refresh.

## Verification

Go tests, vet and Linux race tests cover the product build. Regression tests
include limit increases/decreases in delayed thermal models, active-boost
preservation, qualification-state preservation, more than 90 days of cooling
samples, 90 days of minute history, irregular timestamps, same-timestamp bursts,
log rotation through an existing stderr descriptor, atomic-heartbeat readers,
connection-slot release, cancelled/overlapping reports, oversized reports and
slow browser responses not holding the safety mutex.

The Chromium stress run used 1,950 repeated refresh rounds with charts, reports
and navigation. Garbage-collected heap snapshots after successive 600-round
batches were 2,768,608 / 2,759,768 / 2,759,620 bytes: they plateaued rather than
growing with elapsed rounds. Final retained heap was 116,196 bytes above the
warm baseline, and DOM node count decreased from 418 to 354 after clearing the
last report. Fifty concurrent chart callers shared one request. A simulated
24-hour hidden period issued no polling requests; repeated page suspension /
restoration retained exactly three periodic tasks. This is an accelerated
browser check, not a claim of months of real browser uptime.

The existing desktop/mobile regression checks cover chart ranges/Auto expiry,
notifications, cooling autosave, algorithm/performance controls, live cards and
light/dark layout. The live verification below supplements those simulated checks.

## Scope and remaining limits

Goldshell's stock mining engine, management daemon, kernel, driver and their own
logs remain vendor software. This review bounds this mod's readers and storage;
it does not establish that every vendor process or log is bounded. For example,
the stock mining log was approximately 2.8 MB during review; reading its tail
avoids copying that file into controller memory. Do not delete vendor logs or
alter firmware as a substitute for diagnosing growth.

Safety supervision and accepted work must still be monitored on the actual
hardware. Memory use can fluctuate with Go/JavaScript garbage collection and
request sizes; a short soak cannot certify long-term hardware reliability.

## Live deployment and resource observation

On the original KD BOX at `192.168.86.181`, the updated binary and all dashboard
assets were verified against the build on ports 80 and 8088 after a controlled
application pause. There was no firmware write or reboot. Normal Resume
completed calibration and reached all 16 chip readings at 97.1 seconds.

The actual browser verified **GitHub repository** / **SPECTRE WORKS Store** links,
70→75°C slider autosave and cleared reserve, all-chip diagnostics, System and
Network reads, both dashboard ports and a 390-pixel mobile layout. The owner’s
automatic mode, qualified 755 MHz ceiling, 430 mV voltage, 30% normal-mode fan
minimum and selected 75°C limit were preserved. At the clock ceiling, both
fans remained at 100% as automatic mode requires.

The supervised run lasted 397.3 seconds, including more than
five minutes after telemetry was ready. It captured 144 complete chip reports;
the highest chip was 71°C. The miner reported 37 accepted shares and DATUM
35 at completion; their independently sampled totals can differ briefly.
There was new accepted work at both after startup.

During the observation, controller RSS was 9,328–10,200 KiB
(first 9,328, last 10,200), with 10–10 threads and
10–12 file descriptors (last 10). Temporary browser connections
closed after the UI test; no descriptor or thread accumulation was observed.
Original password/configuration/approved-setting hashes matched at completion.
The temporary QA guard was disarmed and the miner was left hashing. These are
short-soak observations, supplemented by the accelerated tests above.
