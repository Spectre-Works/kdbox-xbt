# First installation / local-LLM handoff

## Goal and boundaries

Validate KD BOX XBT Mod on an **operator-owned original Goldshell KD BOX** on a
trusted LAN. The fork passed a live installation and reboot check on one unit.
Follow this checklist and record results for the device under test.

Do not expand the supported-device allowlist, disable thermal limits, bypass SSH
identity checks, discard an automatic recovery journal, overwrite a safety-stop file,
change voltages beyond policy, or flash/downgrade firmware to make a test pass.
Do not scan unrelated networks. Obtain owner authorization for bounded, read-only
discovery on directly connected private subnets; let the owner select and confirm
the miner. A manually supplied private IP remains available.
Do not upload passwords, private backups, or raw configuration/diagnostics to an
LLM service, GitHub, or support. The operator should enter secrets locally.

## 1. Prepare without modifying the miner

Prepare the original KD BOX on stock **Goldshell 2.2.1** before launching the
installer. Avoid 2.2.4 because its stock management daemon closes SSH on boot
and removes the 2.2.3 SSH enable mechanism. [Stock firmware and SSH](customer-firmware.md)

Have the miner's existing SSH and dashboard passwords ready. Confirm ownership,
recovery access, power and cooling in the installer. If SSH is closed, the
installer checks the stock 2.2.1 identity and starts SSH through the normal
dashboard maintenance request over HTTP on the trusted LAN.

Record the fork commit/version and the local build/test results for the package.
Read `README.md`, `docs/architecture.md`, and the
upstream evidence in `docs/firmware-compatibility.md`. Check the product ZIP's hash.
Run `python tools/build.py` for a source checkout; merely running `go build` does
not create the verified installer payload or complete notice set.

Use the prepared package launcher; it obtains an isolated Python runtime and
Paramiko if needed, then opens the local browser wizard. Tk is not required.
See [host setup](installer-hosts.md) for platform restrictions and manual Python.
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

Discover and select the miner, then confirm its IP and current dashboard password.
The stock 2.2.1 SSH default is used unless the owner supplies their changed SSH password. Check the displayed
SSH fingerprint against a trusted prior connection or locally verified identity.
The owner must accept the wizard's risk warning; do not click through it blindly.
The installer uses the authenticated SSH session to verify the model, board, CPU
architecture, reported and on-disk version, and pinned stock `minerd` hash. File
transfers and installer API requests use the same SSH connection.

Choose **Back up & install KD BOX XBT** once. Record the new backup directory.
Verify that `manifest.json` and `device.json` exist and the backup is private.
New snapshots include managed assets, controller/scripts and configuration;
they are not a full disk/bootloader/vendor-firmware image. Do not delete old backups.

If installation reports failure after changes began, **do not immediately retry
or reboot**. Record the redacted error, check reachability and inspect the backup.
A lost response does not prove a write failed. Use the recovery guidance below.

## 3. Validate the installed interface

Open `http://MINER-IP/` and hard-refresh to clear old cached assets. Check all five
pages, title, favicon, dark/light themes, product version/short commit, and the license link.
The embedded UI at `http://MINER-IP:8088/` should agree with the port-80 copy.
Unlock with the miner dashboard password, then lock/unlock again. Check saved pools
and priority without changing them first. Do not put ports 22/80/8088 on the Internet.

Confirm existing settings and the original `legacy.html` survived. The runtime
uses `/usr/config/kdbox-xbt` and `/etc/init.d/S99kdbox-xbt`. Do not rename them
manually or run competing controller copies from another directory.

## 4. Verify XBT mining, not just a green dashboard

Select **blake2b(XBT)** only after the owner confirms an appropriate pool, payout
address/worker and pool settings. Confirm current and average hashrate, fresh
telemetry, plausible fan RPM, and temperatures. Record a rising accepted-share
counter and confirm worker activity at the chosen pool. A successful installer
startup check is not proof of accepted mining work or stable temperatures.

Remain at the conservative confirmed settings initially. Observe a physically
supervised thermal run long enough to see stabilization in the actual environment;
record duration and ambient conditions. Keep any published measurements specific
to the tested unit and test period.

## 5. Exercise recovery conservatively

Only after stable baseline mining, enable Automatic performance and observe its
bounded adjustments, all chip readings, hardware-error counters and accepted
work. Confirm that turning it off restores the 740 MHz / 430 mV SPECTRE default
without a customer confirmation step. Recovery journals must remain intact.
Do not simulate a thermal fault by obstructing fans or deliberately overheating.

A supervised normal restart should preserve the confirmed pool/algorithm and
settings and return accepted shares. Keep the verified backup available. Tests
involving automatic adjustments across restarts, controller interruption, or network
changes should be separate, explicitly planned steps after baseline validation.
Change only one variable at a time and stop on unexpected telemetry or access loss.

## 6. Restore / stop conditions

The recovery list shows snapshots under `~/KDBoxXBT/backups/`. Choose a complete
schema-3, schema-4, schema-5, schema-6 or schema-7 backup from
this exact miner and matching firmware: MAC, SSH host key, firmware and checksums
are checked before writes. Never edit those identifiers to force a restore.
A source payload is not needed to restore an existing backup, but Python
requirements and supported-device access still are.

Snapshots restore the matching controller, scripts and web assets. Schema 4
also records the locally bundled fonts and saved algorithm selection.
Restoring an original-stock snapshot disables the
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
exist; automated test results alone remain software-only evidence.

## Using the recovery wizard

Launch the installer and connect to the same miner. Complete startup/dashboard files
and an identified responding controller open the dashboard/recovery page without
reinstalling. A configuration file alone is not proof that the mod is running.
An incomplete or nonresponding installation opens a separate **Back up & repair
KD BOX XBT** review. Repair requires a complete verified payload, explicit owner
confirmation and a new private snapshot. It preserves the saved configuration,
clock ceiling, approved settings and algorithm. Existing safety stops or pending
adjustments block application writes and must be inspected first.

### After an original factory reset

Observed on stock 2.2.1: factory reset removes `/usr/config/kdbox-xbt` and
reinitializes native settings, but retains `/root/yottamc`, `/etc/inittab` and
the custom startup hook. The web page survives while the controller and scripts
it calls are absent. Native mining remains held by the missing launcher, and
SSH may need to be enabled again using the installer’s stock maintenance flow.
This is recoverable over SSH on the supported base; no firmware write is needed.

Use **Back up & repair KD BOX XBT**. Surviving private settings remain protected.
If the private configuration was erased, repair creates a new persisted recovery
pause before provisioning, holds the mining processes with full cooling, and
installs defaults. A completely absent native pool list is initialized empty;
existing pool files are never replaced by this initialization. Successful repair
requires readback of the recovery pause. Add/check pools, algorithm, Wi-Fi and
cooling, inspect the hardware, then explicitly resume. Repair cannot know whether
the reset erased a previous safety stop and must not silently start hashing.

Avoid legacy factory reset. The dashboard warns before original maintenance and
shows recovery instructions when its controller is unreachable. These are user
interface mitigations, not a block on the stock HTTP reset endpoint. Automatically
recreating credentials or silently restoring mining from a second hidden copy
would defeat the owner’s reset intent and could discard fault context. Keep a
private off-device backup. The dashboard’s **Factory reset — keep the mod** action clears owner settings
and restores factory credentials using a persisted journal. It retains the mod
and genuine fault records, then reboots for fresh owner setup.

The 2026-11-13 recovery test on one original KD BOX / MCB_V5_4 confirmed this
failure mode after an owner-initiated legacy reset. Repair over SSH restored the
controller, XBT selection and an empty readable pool list, with a persisted
recovery pause. Password records matched the post-reset snapshot; no firmware
write, reboot or native management restart was needed. The installer then
recognized the working build. Desktop/mobile checks covered offline guidance,
reconnection and the maintenance warning. Mining was left paused; this test
does not establish post-reset hashing or accepted shares.

Installation success confirms the identified dashboard, including while stock fan
calibration is in progress. It does not assert mining or accepted shares. Supervise
startup separately and verify all chip temperatures and pool acceptance.

Expand **Restore a recovery backup**, select a complete snapshot from this
computer's backup folder (normally `~/KDBoxXBT/backups/`),
and explicitly confirm the restore/restart. A copied snapshot must remain intact,
including `device.json`, `manifest.json` and all original subdirectories. Never
edit hashes to force a backup through the device/firmware checks.

For installation trouble or guidance, use [GitHub Issues](https://github.com/Spectre-Works/kdbox-xbt/issues).
SPECTRE WORKS offers limited DIY support on a use-at-your-own-risk basis; do not
attach credentials, private snapshots or raw configuration.

## 2026-11-12 live recovery validation

The fixed installer source was exercised on one original MCB_V5_4 KD BOX after a
stock-2.2.1 reflash retained its private mod files but removed the dashboard and
startup hook. Live browser checks covered discovery, default SSH authentication,
identity acceptance/cancellation, explicit repair, verified backup, repeat
connection during calibration, overwrite prevention and recovery confirmation.
A verified post-repair snapshot was then selected in the UI, restored and rebooted.

All 16 chip readings and new accepted miner/DATUM shares returned before and after
restore. Original password-file/configuration hashes, pool settings, algorithm,
75°C cooling limit, disabled Automatic performance and 740 MHz / 430 mV operating
point were preserved. Linux/Chromium desktop/mobile checks and 77 Python tests
passed. This is evidence from one unit and finite test windows; other hosts and
units require their own checks. The published 1.0 package was not replaced.

The test host widened permissions under its home folder after creation. Test
snapshots were secured outside that folder and the final installer additionally
checks completed snapshot permissions before application writes. The documented
`KDBOX_XBT_BACKUP_DIR` override was exercised with that private folder.

## Persistent recovery launcher and setup pauses

New installs and application updates migrate the mining boot entry to
`/root/kdbox-xbt-recovery.sh mining`. The startup hook also starts its singleton
`watch` mode. This small script is outside the partition erased by stock 2.2.1’s
factory reset; on the pinned model/board/base it keeps SSH available. If the
runtime is missing it holds the native supervisor/engine and commands full
cooling. Provisioning, authentication, thermal checks and the persisted pause are
still enforced by the regular launcher/controller. No credential copy or automatic
configuration restoration is embedded in the recovery script.

Schema 7 private backups include this launcher. Schema 3–6 remain accepted with
their original allowlists. Updates back up the old boot files, stage replacements,
verify the deliberate boot migration separately from protected settings, and
restore the previous application/boot files on verification failure. A stock
firmware reflash can overwrite the recovery launcher; it is not firmware recovery.

Persisted pauses report their kind. Only an exact known **pool-setup** record
(including an older record with the same exact setup reason) is eligible for
automatic resume. A valid saved pool, managed configuration and absence of
performance, network or mining-reset journals are required. Missing/corrupt records,
explicit faults, firmware/algorithm constraints and inspection pauses block it.
The stop record is checked again before clearing. A failed automatic start becomes
an inspection-required fault, never a repeated retry loop.

**Factory reset — keep the mod** is authenticated and requires typing
**FACTORY RESET**. This deliberately clears personalization, including passwords,
network profiles, pools, cloud credentials, labels, tuning and history. Both login
passwords return to the documented stock defaults. SSH authorized keys are removed;
native RS256 signing keys are replaced and mod sessions are invalidated. The native
settings partition is cleared except for the application and its reset/fault state.
Defaults are rebuilt from the pinned device templates and the defined mod policy.
Known root credential/history caches and network overrides are removed too.

Vendor writers are held during the erase and restarted afterward to discard their
cached state. Partial credential provisioning retains a private fallback only
until reset completion, allowing either the old or new native login to recover
from an interrupted reset; no old password remains after successful completion.
Genuine/unknown faults stay latched. Benign setup/recovery/update pauses become a
fresh owner-setup pause after the explicitly confirmed owner reset succeeds.

The owner reset uses a version-2 journal. A failed/ambiguous erase requires an
explicit retry. Reboot is requested once; a changed boot identity and completed
factory credential provisioning are required before the journal is removed.
Wi-Fi disconnects and the IP may change: reconnect by Ethernet and discovery.
Previously accepted version-1 mining-only journals remain non-destructive and
are never silently upgraded into owner-data erasure.

This is managed-state cleanup, not secure flash erasure, a full OS reimage or a
remote-cloud-account unlink. Off-device backup copies remain under owner control.

### Earlier mining-only reset validation — 2026-11-13

Before the owner-reset correction, on the recovered, already-empty test unit, the authenticated dashboard mining
reset restored the defined defaults and completed its journal while retaining the
original recovery record byte for byte. Password and network records remained
unchanged. Browser checks covered confirmation, setup-pause wording, hidden
manual resume for setup, retained inspection controls for faults and mobile layout.
A supervised reboot then returned SSH, the controller and its watchdog through
the new startup wiring, preserving the protected records and recovery pause.
Mining was not resumed. Automatic pool startup, changed/corrupt stops, pending
adjustments, failed startup and interrupted/failed reset cases were checked with
automated native-service fixtures; this session did not claim live accepted shares
for an automatic pool resume or repeat a destructive factory reset on the unit.

### Full owner-reset validation — 2026-11-13

The full factory reset was exercised through the live dashboard on the original
KD BOX / MCB_V5_4 at 192.168.86.203, stock 2.2.1. The test seeded non-default web
and SSH passwords, an accepted SSH authorized key, an unreachable test pool,
an inactive saved Wi-Fi profile, persistent static-IP/DNS configuration, cloud
and device-label markers, tuning preferences, notes and history. Mining stayed
held during preparation. Static configuration files were seeded without changing
the active Ethernet link before the reset.

After the confirmed reset and a verified new boot, both factory passwords worked;
the test passwords, SSH key and old mod session were rejected. The pools and owner
markers were absent, DHCP/automatic DNS were enabled, Wi-Fi was disabled with no
saved connection, cloud credentials were empty, and tuning returned to 740 MHz /
430 mV, 75°C, Automatic performance off and a 740 MHz initial clock ceiling.
The mod, watchdog and persistent recovery launcher remained operational. Both
HTTP ports served the expected assets; desktop/mobile checks covered factory
login, fresh pool/network setup and automatic setup-pause guidance. The installer
recognized the exact running build as Already installed. The miner was left locked,
with no configured pool and no hashing, ready for owner setup.

The first live pass exposed stock minerd restoring its private signing key from
`/root/settings/key.pem` at startup. Factory reset now rotates both the persistent
source templates and runtime signing keys; a regression test covers this behavior.
The corrected live run verified that old JWT signatures matched the old key but
failed against the new key, and fresh login tokens verified against the new key.
Stock read-only debug responses are not an authentication/revocation test. The
mod's old authenticated session was checked separately and rejected with HTTP 401.

Automated filesystem/native-service fixtures additionally cover genuine fault
retention, partial failures, no automatic replay of ambiguous erases, and reboot
identity. This finite test does not establish secure flash erasure or qualify
other hardware. Private recovery snapshots and test evidence remain off-device;
temporary test credentials and client keys are removed after verification.
