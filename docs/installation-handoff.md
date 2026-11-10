# First installation / local-LLM handoff

## Goal and boundaries

Validate KD BOX XBT Mod on an **operator-owned original Goldshell KD BOX** on a
trusted LAN. The fork has not yet been hardware validated. Follow this checklist
in order; record observations instead of assuming upstream reports prove this
build works. No hardware was contacted by the repository rebranding work.

Do not expand the supported-device allowlist, disable thermal limits, bypass SSH
identity checks, force a pending trial to disappear, overwrite a safety-stop file,
change voltages beyond policy, or flash/downgrade firmware to make a test pass.
Do not scan unrelated networks. Use the specific IP supplied by the operator.
Do not upload passwords, private backups, or raw configuration/diagnostics to an
LLM service, GitHub, or support. The operator should enter secrets locally.

## 1. Prepare without modifying the miner

Record the fork commit/version and whether the package came from a successful
workflow or was built locally. Read `README.md`, `docs/architecture.md`, and the
upstream evidence in `docs/firmware-compatibility.md`. Check the product ZIP's hash.
Run `python tools/build.py` for a source checkout; merely running `go build` does
not create the verified installer payload or complete notice set.

Confirm Python 3.12+, Tk, Paramiko, and the GUI work on the installation computer.
Prepared packages already have the ARM controller; Go is not needed to run them.
Do not run the ARM executable on the desktop computer. Keep the entire extracted
package together. Do not edit a deployed UI/script and then reuse a stale payload.

Read the miner's stock UI: verify model, firmware and board against
`controller/compatibility.json`. Confirm the board over SSH if the stock UI omits
it. Record its current pool/worker, selected algorithm, frequency/voltage, fan RPM,
temperatures and accepted-share counter privately. Save existing owner backups.
Ensure power and ventilation are stable and someone can physically inspect it.
Do not proceed if it is already overheating, inaccessible, or reporting faults.

## 2. Connect and install once

Use the actual miner IP and separate SSH/dashboard credentials. Check the displayed
SSH fingerprint against a trusted prior connection or locally verified identity.
The owner must accept the GUI's risk warning; do not click through it blindly.
The installer may enable the stock SSH service only on supported firmware after
owner authentication. Initial stock HTTP authentication is unencrypted: trusted
LAN only. File transfer and post-SSH installer API requests use SSH.

Choose **Back up & install KD BOX XBT** once. Record the new backup directory.
Verify that `manifest.json` and `device.json` exist and the backup is private.
New snapshots include managed assets, controller/scripts and configuration;
they are not a full disk/bootloader/vendor-firmware image. Do not delete old backups.

If installation reports failure after changes began, **do not immediately retry
or reboot**. Record the redacted error, check reachability and inspect the backup.
A lost response does not prove a write failed. Use the recovery guidance below.

## 3. Validate the installed interface

Open `http://MINER-IP/` and hard-refresh to clear old cached assets. Check all five
pages, title, favicon, dark/light themes, product version, and the license link.
The embedded UI at `http://MINER-IP:8088/` should agree with the port-80 copy.
Unlock with the miner dashboard password, then lock/unlock again. Check saved pools
and priority without changing them first. Do not put ports 22/80/8088 on the Internet.

On a previous Paperclip installation, confirm existing settings and original
`legacy.html` survived. Internal `/usr/config/paperclip`, cookie and startup names
are intentionally retained. Do not rename them manually or run competing
controller copies from another directory.

## 4. Verify XBT mining, not just a green dashboard

Select **blake2b(XBT)** only after the owner confirms an appropriate pool, payout
address/worker and pool settings. Confirm current and average hashrate, fresh
telemetry, plausible fan RPM, and temperatures. Record a rising accepted-share
counter and confirm worker activity at the chosen pool. A successful installer
startup check is not proof of accepted mining work or stable temperatures.

Remain at the conservative confirmed settings initially. Observe a physically
supervised thermal run long enough to see stabilization in the actual environment;
record duration and ambient conditions. Do not claim a model-wide benchmark from
this single unit. Firmware 2.0.6 has tighter software limits; consult its notes.

## 5. Exercise recovery conservatively

Only after stable baseline mining, test a small **downward** clock trial within
existing bounds. Let an unconfirmed trial expire and verify restoration of the
previous live values. Then test confirmation only after a new accepted share.
Do not simulate a thermal fault by obstructing fans or deliberately overheating.

A supervised normal restart should preserve the confirmed pool/algorithm and
settings and return accepted shares. Keep the verified backup available. Tests
involving pending trials across restarts, controller interruption, or network
changes should be separate, explicitly planned steps after baseline validation.
Change only one variable at a time and stop on unexpected telemetry or access loss.

## 6. Restore / stop conditions

The restore picker supports new and old backup locations. Choose a backup from
this exact miner and matching firmware: MAC, SSH host key, firmware and checksums
are checked before writes. Never edit those identifiers to force a restore.
A source payload is not needed to restore an existing backup, but Python
requirements and supported-device access still are.

Schema-2 snapshots restore the matching controller/scripts and web assets and
remove only allowlisted web files recorded absent. Old schema-1 backups remain
accepted but did not capture the full UI/controller; do not treat them as a complete
software-version rollback. Restoring an original-stock snapshot disables the
custom startup hook; unused runtime files may remain inert. Never delete state
files while a controller is still operating or use factory reset as an experiment.

Stop and inspect after a safety stop, stale/missing telemetry, failed checksum,
wrong architecture, changed identity, unsupported firmware, interrupted write, or
failed startup. Preserve the original fault details. Recovery may require physical
access or specialist assistance. Software checks do not guarantee recovery.

## Record the result

Use a local test record containing commit/version, exact model/board/firmware,
installation outcome, actual XBT accepted-share evidence, cooling observations,
reboot/rollback outcomes, upgrade/restore outcome, and unresolved faults. Redact
secrets before sharing. Promote hardware support claims only after those results
exist; CI results alone remain software-only evidence.
