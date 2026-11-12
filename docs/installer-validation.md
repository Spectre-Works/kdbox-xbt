# Guided installer and invalid-nonce policy validation

Recorded 2026-11-11 for the 1.0 preview. Results describe these checks and one
original Goldshell KD BOX, not certification for other miners or host platforms.

## Automated checks

- Go tests, vet and Linux race tests passed with the product's HTTP build tag.
- 68 Python tests passed, including device/base gates, complete recovery manifests,
  credential redaction, bounded read-only discovery, owner and SSH identity
  confirmation, first-install completion, existing-install protection, partial
  install recovery, and local-server token/Host/Origin/body restrictions.
- Node syntax checks passed for the dashboard and installer JavaScript.
- Invalid-nonce regressions cover default-mode isolated and large error counts,
  missing quality counters, tolerating an isolated error during automatic
  qualification, excessive-error rollback, established-clock backoff with a
  persisted lower ceiling, counter baselines, observation windows, and the
  minimum clock not requesting an error-induced stop. Thermal tests still pass.

## Installer host and browser checks

A fresh Linux x86_64 setup in a directory containing spaces downloaded and
checksum-verified uv 0.13.7, obtained managed Python 3.13.18, installed Paramiko
and dependencies, opened the local server with a valid prepared payload, and
shut down through its browser API. No system runtime or administrator setup was
used. All six launcher archive hashes matched Astral's release metadata; the
Windows ZIP's `uv.exe` extraction layout was checked.

Chromium exercised the complete simulated first-install flow: discovery,
readable validation errors, factory-default SSH selection, explicit identity
confirmation, verified review, refresh/session recovery, final installation
confirmation, successful completion, correct dashboard link and clean shutdown.
Desktop and 390-pixel mobile layouts were inspected without horizontal overflow.
The miner-writing adapter was simulated for this fresh-install browser test.

Windows/macOS interface outputs are covered by discovery tests and have dedicated
launchers. Native Windows/macOS execution was **not** performed in this Linux
session. Platform restrictions and a manual Python fallback are documented in
[host setup](installer-hosts.md); no claim of native validation on every OS is made.

## Live miner checks

The real wizard automatically discovered `192.168.86.181`, selected it only after
confirmation, displayed the known SSH fingerprint, verified stock 2.2.1 / MCB_V5_4 /
ARMv7 and the pinned management hash, and detected the existing installation.
The existing-install flow offered the dashboard/recovery path without reinstalling.
Credential/configuration hashes and native mining process IDs remained unchanged.

The updated controller and complete dashboard asset set were then installed using
a controlled mining pause, a verified private schema-5 snapshot, atomic staging,
matching running-binary/asset hashes on ports 80 and 8088, and rollback checks.
No stock firmware, reboot, pool, password, cooling limit or qualified clock-limit
change was made. The GitHub Issues help link was included on both dashboard ports.

The first observation was interrupted by the independent QA guard's heartbeat
file race: a non-atomic heartbeat rewrite could be read while empty. Its saved
stop identified the test guard, not a controller thermal or invalid-nonce stop.
The harness was corrected to replace its heartbeat atomically. The recorded
QA stop was inspected and normal supervised Resume was repeated; it was not
silently removed by the installer.

The repeat resume succeeded on its first attempt. Stock calibration reached the
first-readings phase at 86.6 seconds and all 16 chip temperatures at 98.9 seconds.
Over the following 90 seconds, 45 complete chip reports were captured; the
highest recorded chip was 67°C. The miner and DATUM each reported 14 accepted
shares at completion, with new accepted work after telemetry became ready.
The existing 740 MHz / 430 mV default mode, 75°C selected limit, 30% fan minimum,
755 MHz qualified ceiling, original password files and saved settings matched.
The QA guard was disarmed after verification and the miner was left hashing.

Invalid nonces were not deliberately induced on the hardware. Their revised
control behavior is established by regression tests; this short live check
verifies deployment, calibration, temperatures and accepted work.
