# Architecture and recovery

The Python browser installer uses Paramiko on Windows, macOS and Linux. A local
127.0.0.1 server uses a random per-session token, strict Host/Origin checks and
no-store responses. It never serves local files other than its three static UI
assets. Private-network discovery is bounded and read-only; SSH identity requires
explicit owner confirmation. Device identification
must match the adapter before writes. Backups include hashes and permissions;
they contain private configuration and must not be committed or shared.

The Go ARM controller lives in `/usr/config/kdbox-xbt`, outside the small root
partition. The native mining binaries and web API remain installed. Our UI assets
replace the index on the original port 80; the browser talks to the authenticated
local control service on port 8088. CORS permits only the same hostname on the
expected local ports. No remote scripts, analytics, or cloud account are used.
Do not expose these legacy HTTP services directly to the Internet.

`approved.json` is the last verified clock/voltage operating point. `pending.json`
is an internal recovery journal written atomically before a performance change.
At startup the launcher restores the previous hardware parameters on disk before
starting the native mining supervisor. The controller then completes API recovery.
`halted.json` retains a thermal, fan, or telemetry stop across controller
restarts. Inspect the miner before using **Resume mining**.
`network-pending.json` similarly retains the previous network configuration.

On stock 2.2.1, an XBT boot or resume gives the stock supervisor up to four
minutes to calibrate the fans while neither mining engine is running. Resume
clears the stock PID file if its recorded process is gone. As soon as a mining
process appears, the controller commands full fan speed, checks the 3,000 RPM
floor, and allows 25 seconds for the first valid temperature report. Readings
that reach a thermal limit stop mining during that window. After the first
valid report, the regular telemetry checks apply immediately.

During normal mining, automatic cooling uses a 75°C hottest-chip limit
by default with a 30% duty minimum. The saved `chip_target` in the existing
private configuration accepts 70–80°C through the Cooling panel. The slider
saves after one second without adjustment and reports success or failure.
Cooling limit changes apply without restarting the engine.

Normal fan control uses the median of eleven readings (30 seconds at the normal
cadence, covering repeated stock telemetry excursions) and a bounded trend from
the preceding 30 seconds to anticipate warming over the next 30 seconds. A
poorly fitting, noisy trend is weighted down rather than fully extrapolated. A
falling trend never justifies extra reductions. Increases start when the
projected controlling temperature exceeds its limit by 0.5°C, and stop at the
limit. Forecast-only increases are one or two percentage points followed by
15 seconds of settling. Confirmed hot readings can increase four to ten points
per three seconds; readings close to the raw boost limits bypass confirmation.
Quiet readings retain hysteresis.
Two fresh readings below the longer median cancel stale forecast-driven
increases after an excursion ends. They do not authorize faster reductions.

The hottest-chip objective includes bounded headroom learned from positive
excursions beyond the median's expected lag. This keeps the upper range of
readings near the selected limit, rather than letting a cool median hide hot
excursions. The learned spike allowance is scaled to 50% and capped at 3°C; it blends in at up to 1.8°C per minute,
and only decays after five quiet minutes. A reading within 7°C of the hard chip
limit applies the learned reserve immediately. The raw 85°C full-fan threshold
and 90°C shutdown remain unchanged on stock 2.2.1.

Reductions need fresh trend history and a 20-second cooling dwell. With more
than 8°C of projected headroom, output can fall two points per six seconds;
with 4–8°C, one point per twelve seconds; near the limit, one point per thirty
seconds. Near-limit warming cancels reductions and restarts the dwell, so a
reduction's thermal effect can emerge before more airflow is removed. The
30% normal minimum remains a floor, not a requested running speed.

The separate 15-second exponential temperature filter remains in place for
automatic performance and the sustained-heat protection below.
A raw chip reading at limit +5°C must persist for nine seconds before latching
full output, or the filtered hottest-chip signal must reach that threshold. A
brief noncritical chip excursion therefore does not latch the boost. Chip
readings within 5°C of the firmware peak limit, board readings at 75°C, and
chip averages within 4°C of their limit still command full output immediately.
Full output lasts at least 30 seconds and needs
15 seconds of sustained cooling before release. Raw thermal shutdown checks
run before the filtered controller.

The hashboard temperature also has a 70°C target.
Goldshell defines its normal operating temperature using the second value in
the stock temperature pair. The first value is the chip average. The hottest
chip gets an early sustained-heat boost at 80°C with the default 75°C limit, ahead
of immediate full output at 85°C and the existing 90°C chip thermal stop on
stock 2.2.1. No thermal shutdown threshold uses the filtered readings. Fan
tachometer checks scale with duty, retaining the 3,000 RPM
floor at full output and checking each fan separately. Missing telemetry, a
safety stop, and active sensor initialization command full cooling. Clock and
voltage changes preserve the cooling safeguards. Automatic performance commands
100% fan output and uses the same filtered hottest-chip signal to govern clock
at a fixed 430 mV core voltage. With automatic performance off, normal fan
modulation remains active.

Normal PWM writes stop once the driver reports the requested duty. Changed
output uses a one-point command allowance for the stock driver's asynchronous
rounding, so the reported duty stays at or above the requested floor. Full-speed
protection always writes through; it does not wait for the normal write check.
[Goldshell temperature guidance](https://goldshellhelp.zendesk.com/hc/en-us/articles/10554697577497-Miner-s-normal-operating-temperature)

Hashrate history bootstraps from the stock 288 one-minute samples, discarding
initial zero padding while preserving later zero-rate periods. A separate
collector extends the timestamped history to 1,440 samples without an open
browser. `/tmp/kdbox-xbt-history.json` preserves it across controller restarts
without periodic flash writes; a miner reboot clears this RAM-backed history.
Missing samples remain gaps. Auto selects the smallest of 1h, 3h, 6h, 12h and 24h
that fits the retained history. A manual range expires after five minutes, or
on page reload.

An independent supervisor checks the controller heartbeat. On a stall it pauses
the native mining supervisor, stops mining, forces the fans to full speed and
restarts the controller. It does not stop the web service or SSH. Kernel panic
or a complete hardware freeze is a separate failure mode; power-cycle if needed.
No tuning feature depends on leaving a browser or the installer open.

The native driver still expects the name `blake2b(SC)`. Only the user-facing name
is `blake2b(XBT)`. Changing the driver identifier would break algorithm selection.

Manual tuning and confirmation endpoints are removed. The authenticated
`/api/performance` endpoint accepts only the automatic-mode boolean. The
[automatic performance policy](automatic-performance.md) defines the fixed
default, bounded optimization, internal verification and recovery.

## If an automatic performance adjustment fails

Leave the miner powered and wait for automatic rollback. If the device is fully
unresponsive, power-cycle it once. The previous settings are restored at boot.
If mining remains stopped, inspect cooling and the dashboard fault message before
resuming. Do not delete `pending.json` to force a failed performance adjustment through.

## Restoring the original dashboard

Use **Restore a recovery backup** in the installer. It checks the device MAC, SSH
host key, and backup checksums before restoring and restarting the miner.

For manual recovery, keep your backup directory. Restore its `root/yottamc/index.html` and
`etc/inittab` to the matching device paths with their recorded permissions. Remove
the KD BOX XBT startup hook only after restoring the original startup entry. Then
restart the device. The installer never modifies the bootloader or full firmware.
Do not restore a backup from another miner. For a full settings rollback, restore
the matching backed-up configuration files as well; the manifest records them.

## Boundaries

The new controls protect changes made through KD BOX XBT. Direct SSH, the legacy
maintenance interface, and native API clients can bypass them. Avoid using those
interfaces concurrently for tuning. Wi-Fi controls are conditional on native
hardware support. Upstream hardware evidence covers one original KDBox test unit;
do not infer support for other Goldshell products from the shared UI.

## SPECTRE WORKS fork: packaging and recovery changes

The product is KD BOX XBT Mod / KD BOX Control. The runtime directory is
`/usr/config/kdbox-xbt`; the startup hook is `/etc/init.d/S99kdbox-xbt`.
The session cookie is `kdbox-xbt`, and diagnostic/heartbeat files use
`/tmp/kdbox-xbt-*`. This fork does not retain upstream runtime aliases or
backup-location compatibility. The native device API and driver paths retain
their vendor-defined identifiers.

`python tools/build.py` generates actual Go toolchain notices, embeds the complete
web payload, cross-compiles the controller, and creates a SHA-256 manifest. Both
the browser wizard and the device installer validate the payload before
installation writes. The wizard reports a missing payload before discovery;
connecting to an existing installation remains available for recovery. Fresh
installation preflight checks the payload again after connecting. Over authenticated SSH, the installer
checks the device identity, stock 2.2.1 version and pinned `minerd` hash.
Obtain code and artifacts from the intended fork and commit.

The default release is editable source plus the ARM controller, with separately
installed desktop dependencies.
The complete license text is embedded in and deployed with the controller/UI.

New backup manifests have version 6 and include the header emblem, dashboard fonts, saved algorithm,
controller binary, startup scripts and managed configuration. Files and the
snapshot directory are owner-only on POSIX systems. On
Windows, verify the user-profile/folder ACLs separately. Backups contain secrets.
Every recovery path is allowlisted and every present file is verified before any
restore write.

Restore accepts complete schema-3, schema-4, schema-5 and schema-6 snapshots.
Schema 6 adds the website PNG favicons, and retains the old favicon path only for
verified backup/restore cleanup. The current application no longer ships it. Restoration removes
allowlisted web assets and saved algorithm files recorded as absent. An original-stock restore disables the custom boot
hook; unused runtime files may remain on disk but should not be started manually.
The snapshot is not a full filesystem image or a backup of the vendor bootloader.

Installation detection checks executable runtime files, startup wiring, dashboard
assets and the controller status identity. Remnants after a stock reflash enter
an explicit repair review rather than claiming the application is installed.
Repair snapshots first and preserves existing configuration and approved/algorithm
files. A stopped or calibrating responding controller remains an existing
installation. Startup readiness does not require mining telemetry, which can
legitimately be absent during stock fan calibration.

Unknown/custom files are not swept from the vendor web directory. The original maintenance page is
preserved. Do not clear a pending performance adjustment or safety stop to force an upgrade.

## Resource budgets and scheduling

The [dashboard endurance review](dashboard-endurance.md) records browser polling
and memory behavior, fixed history/report/log budgets, connection limits and
accelerated long-run checks. Read-only dashboard work releases the safety mutex
before native waits or response serialization. Abandoned reads cancel their
upstream work. The independent watchdog uses an atomically replaced heartbeat;
raw thermal and fan checks retain their existing deadlines.

## Build identity

Release builds embed software version, full Git commit and dirty-state metadata
in the ARM controller. `/api/status` reports these under `software`; the footer
shows the version and seven-character commit. Metadata comes from the running
binary, so installing a prepared ZIP requires no Git checkout on the desktop.
Dirty builds are marked `-dirty`; builds without a source revision are shown as
`unversioned`. Packaging requires a clean source tree at the compiled commit,
matching source/payload hashes and matching Go linker metadata.

## Initial algorithm and confirmed state

New installations select native algorithm ID 1, Bitcoin (XBT), called
`blake2b(SC)` by stock firmware. Before starting the controller, the installer
persists `algorithm.json`, applies the native selection and checks its readback.
A repair keeps a valid saved selection; for older existing configurations without
that file, it preserves the native choice. A controller with no saved preference
also defaults to XBT, but queues it without changing the stock engine while halted.
An explicit saved KDA choice survives restarts. The dashboard's initial selector
is XBT and stays disabled before connection; confirmed controller status then
controls its value. A requested change is not displayed as applied until fresh
controller readback returns. Application success is distinct from mining or share
acceptance, which still require telemetry and pool checks.
