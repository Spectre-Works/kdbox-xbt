# KD BOX XBT 1.0.5

Stock hashboard restarts no longer trigger a clock-limit stop solely because
of their temporary clock ramp. On the pinned stock 2.2.1 XBT base, a newly
observed native reinitialization event permits a fixed 15-second recovery
window, at the expected voltage and no higher than the managed clock.
The controller commands full cooling and keeps temperature, RPM, voltage,
telemetry and watchdog protections active. Unexplained clock changes, timeout
and closely repeated recovery attempts still stop mining.

The dashboard shows **Hashboard restarting** during recovery. Automatic tuning
pauses, and an interrupted qualification restores its previous settings rather
than committing observations from across a hashboard reset. The mining-session
timer excludes the recovery interval.

Hashrate history now records zero during known stops, startup and hashboard
recovery instead of extending a frozen stock history value. New samples use
fresh controller telemetry; unknown readings leave gaps. A valid saved stop
timestamp corrects the known stopped interval while preserving earlier history.
Resuming cannot replay stale stock values over that zero-rate interval. History
remains bounded to 24 hours and 1,440 samples in RAM-backed storage.

## Download and update

Download `KDBoxXBT-1.0.5.zip` and `SHA256SUMS.txt`, verify the checksum, extract
the entire ZIP and run the launcher for your platform. See the
[README](../README.md) and [host setup guide](installer-hosts.md).
`kdbox-xbt-source.zip` contains source only.

The supported base remains the original Goldshell KD BOX, MCB_V5_4 / ARMv7,
on stock Goldshell 2.2.1. Obtain vendor firmware separately; none is bundled.
Updates preserve passwords, pools, network/performance settings and existing
stops. Mining pauses for update and remains paused for review.

Regression coverage includes fresh/stale restart evidence, fixed recovery
deadlines, clock/voltage bounds, repeated restarts, automatic qualification
rollback, zero-rate transitions, retained stop correction, stale telemetry,
concurrent stops during history bootstrap and resume/history retention.

**Use at your own risk.** See the README for installation limits, recovery
guidance and limited DIY support through GitHub Issues.
