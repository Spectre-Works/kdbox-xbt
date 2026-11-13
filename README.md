# KD BOX XBT Mod

**Rebuilt for Bitcoin.** Run XBT (Bitcoin BLAKE2b) mining on a compatible original
Goldshell KD BOX, with the **KD BOX Control** dashboard and guided **KD BOX XBT Installer**.
Software version **1.0.1** · Customer-installed SSH modification.

SPECTRE WORKS-owned software uses [Apache License 2.0](LICENSE); retained upstream
code keeps its [MIT notice](licenses/upstream-MIT.txt). Modified redistributions
must replace the official product branding under the separate [brand policy](BRANDING.md).

This independent SPECTRE WORKS project installs application files over SSH and uses
the mining engine already on your miner. It is not a complete firmware image and
is not developed, endorsed or supported by Goldshell. KDA selection remains available.

## Before you begin

You need:

- An **original Goldshell KD BOX**, board **MCB_V5_4**, ARMv7, running **stock Goldshell 2.2.1**.
  KD BOX II, Pro and other models are not supported.
- A Windows, macOS or Linux computer with a modern browser, Internet access for
  first-time tool setup, and access to the miner's private local network.
- Reliable miner power, unobstructed cooling, your current dashboard password,
  and physical access to supervise the first run. No serial console is required
  for a normal installation; a failed device may still need specialist recovery.
- An XBT-compatible pool and your own payout/worker details. The mod does not
  provide a pool or silently substitute a payout address.

**Use at your own risk.** Installation interrupts mining and changes startup files.
Failures can remove access, damage hardware or brick the miner. Backups, watchdogs
and thermal safeguards cannot guarantee recovery or prevent every failure.
The software is provided without warranty under [LICENSE](LICENSE). Separate seller
warranties and applicable law remain separate.

SPECTRE WORKS provides **limited support to DIY users on a use-at-your-own-risk
basis**. For issues, help or guidance, [open a GitHub issue](https://github.com/Spectre-Works/kdbox-xbt/issues).

## 1. Prepare stock Goldshell 2.2.1

If your miner already reports 2.2.1, continue to step 2. Otherwise:

1. Obtain [firmware-Goldshell-KDBox-2.2.1.cpb directly from Goldshell](https://github.com/goldshellminer/firmware/blob/master/KDA-Miner/Goldshell-KDBox/firmware-Goldshell-KDBox-2.2.1.cpb).
   On that GitHub file page, use **Download raw file**; do not save the HTML page.
   Verify SHA-256:
   `5984af95c4434f2b20987548560263eb0b361674309e541f881551dcca55e87d`.
2. Find the miner's current IP in your router's device list and open its dashboard
   at `http://MINER-IP/`. Unlock it with your current password, open **System**,
   choose the downloaded 2.2.1 `.cpb` in the firmware-update field, then choose
   **Update**. Wait until the update and restart finish; never interrupt a firmware
   write. [Goldshell's dashboard firmware guide](https://goldshellhelp.zendesk.com/hc/en-us/articles/16805936980633-Upgrade-Miner-Firmware)
3. Confirm that the stock dashboard reports **2.2.1**. Its IP can change after a restart;
   the installer searches your local network for it.

**We do not include, mirror, download or install Goldshell firmware in our tooling.**
You obtain the vendor image separately. Application backups are not firmware backups.
See [firmware preparation and verification](docs/customer-firmware.md).

**Do not upgrade to 2.2.3 or 2.2.4 for this mod.** Both stop SSH at management startup
and cannot restart it through the ordinary dashboard maintenance endpoint used
by this installer. 2.2.4 also removes the 2.2.3 service-login branch and changes the
kernel, mining programs and fan controller. Only the pinned **2.2.1** base is accepted
for installation. [Firmware evidence](docs/firmware-compatibility.md)

## 2. Open the installer

Download **[KDBoxXBT-1.0.1.zip](https://github.com/Spectre-Works/kdbox-xbt/releases/download/v1.0.1/KDBoxXBT-1.0.1.zip)**
and **[SHA256SUMS.txt](https://github.com/Spectre-Works/kdbox-xbt/releases/download/v1.0.1/SHA256SUMS.txt)**
from the [1.0.1 release](https://github.com/Spectre-Works/kdbox-xbt/releases/tag/v1.0.1).
Verify the ZIP checksum before opening the installer.

The product ZIP includes the prebuilt ARM controller and editable installer source.
GitHub's **Code → Download ZIP** and automatic **Source code** downloads are source
archives, not ready-to-install packages: they lack the built controller.
Maintainers can produce the product ZIP using the build commands below.

Check the product ZIP's SHA-256 against `SHA256SUMS.txt`, then **extract the whole
ZIP** into a writable folder. Do not run the installer from inside the ZIP.
Checksums detect corruption; obtain both files from a trusted project source.
[How to check a download](docs/installer-hosts.md#verify-downloads)

| Your computer | Open this file |
| --- | --- |
| Windows 10/11, x64 or ARM64 | Double-click **install.cmd** |
| macOS, Intel or Apple silicon | Double-click **install.command** |
| Linux, x64 or ARM64 with glibc | Open a terminal in the extracted folder and run **sh install.sh** |

The launcher prepares Python and its libraries locally if needed, then opens a
JavaScript-capable browser window. It needs no administrator account, global Python installation,
Tk, Go or Node.js. First launch needs Internet access to GitHub and Python package
sources. Later launches reuse the local environment. Leave the launcher window
open until installation finishes. If the browser does not open, use the local URL
shown in that window. Do not share that session URL.

Operating-system security prompts, locked-down work computers, other architectures,
missing host utilities or no graphical browser can need manual setup.
[Platform setup and troubleshooting](docs/installer-hosts.md)

## 3. Find, confirm and install

1. Confirm that you own/administer the network, then choose **Find KD BOX miners**.
   Select your miner from the results and confirm that it is yours. Searching is
   read-only and limited to small private local subnets. If discovery misses it,
   use **Look up an IP address** with the IP shown by your router.
2. Enter your **current dashboard password** (Goldshell default **123456789**).
   Leave the advanced SSH password field blank for the public stock 2.2.1 root
   default; enter it only if you previously changed it. **Neither password is changed.**
   If necessary, the tool starts SSH through stock dashboard maintenance after
   checking the model and firmware. This initial request uses HTTP on your trusted LAN.
3. Confirm the miner's SSH identity. Compare the fingerprint with a trusted earlier
   connection when available. Discovery is not proof of identity.
4. Review the verified IP, stock base and board. The installer also checks ARM
   architecture, the on-device version file, the exact stock mining-management
   binary hash, current credentials and free space before offering installation.
5. Confirm ownership, power, cooling, recovery access and modification risk, then
   choose **Back up & install KD BOX XBT**. Keep power on and wait for completion.
   The tool saves and verifies a private recovery snapshot before application writes.

A complete installation with a responding controller opens a dashboard/recovery
flow instead of reinstalling. If a stock reflash or interrupted installation left
files behind but removed the dashboard/startup, choose **Back up & repair KD BOX XBT**
after reviewing the separate repair confirmation. Repair creates a new private
snapshot, restores application/startup files and keeps the saved configuration,
qualified clock ceiling, cooling limit and algorithm. It does not clear safety
stops; inspect any existing stop before proceeding. A missing payload still allows
backup recovery, but cannot repair application files. Passwords are unchanged.
New installations select Bitcoin (XBT); repairs retain the saved algorithm choice.
When installation switches a miner from KDA to XBT, its old KDA pools are included
in the private backup and cleared before XBT is applied. Existing XBT pools are
retained. A miner with no configured pools waits for pool setup with mining paused;
add an XBT pool and resume when ready. The installer never silently resumes a safety-stopped miner.

If installation stops after writes began, **do not reboot or blindly retry**.
Keep the reported backup path and follow [recovery guidance](docs/installation-handoff.md).

## 4. Configure and verify XBT mining

Choose **Open miner dashboard**, or visit `http://MINER-IP/`.

1. Unlock controls using your unchanged dashboard password.
2. In **Mining Setup**, add or check the XBT pool address, priority and payout/worker. Select
   **Bitcoin (XBT)** selection. New installations select it automatically; apply
   a different algorithm only if intended. Resume mining if the dashboard asks
   for an inspection/confirmation. Do not clear a safety stop without inspection.
3. Supervise startup. Stock fan calibration can take a few minutes before normal
   telemetry appears. Confirm both fan RPMs, all chip temperatures, hashrate,
   increasing **accepted shares**, and matching activity at your pool.
4. New installations use **SPECTRE defaults** (740 MHz / 430 mV), a **75°C**
   hottest-chip limit, and **Automatic performance off**. After a stable supervised
   run, optionally enable **Automatic performance**. A dashboard loading successfully
   does not establish that the miner is hashing or that shares are accepted.

Keep management on a trusted LAN. Do not expose ports **22, 80 or 8088** to the
Internet; the miner's management HTTP is unencrypted.

## Backups and recovery

Private application snapshots are saved automatically under
`~/KDBoxXBT/backups/` (your user profile on Windows). Keep them on this computer,
with access limited to your account; on Windows check the folder's user ACLs.
For a shared home folder or host permission restrictions, select another private
folder using [the backup-folder setting](docs/installer-hosts.md#private-backup-folder).
They can contain passwords, pool credentials, Wi-Fi secrets and identifying data.
**Never post a backup, credentials or raw configuration to GitHub.**

To restore, launch the installer, discover/connect to the same miner, open
**Restore a recovery backup**, select the snapshot, and explicitly confirm the
restart. The tool checks the firmware, device identity and complete backup hashes
before writes. Restore requires SSH and a working base; it cannot repair every
boot, storage or firmware failure. [Recovery details](docs/installation-handoff.md)

## Dashboard behavior

The hottest-chip limit is **75°C by default**, adjustable to **70–80°C**,
with a **30% fan minimum**, smoothing, hysteresis, trend prediction and thermal
headroom. The learned spike reserve is half its original amount, capped at
3°C; raw overheating protection remains immediate. Changing the limit resets reserve/forecast history learned at the previous
setting while preserving raw thermal protection and any active boost. Both fan
percentages/RPMs and live clock/voltage appear on Overview. The **Mining session** row shows continuous time since the last return to Mining active; it
is tracked by the controller and survives browser refreshes. The footer shows
version **1.0.1** followed by the short source commit; include both in issue reports.
Hashrate history offers Auto, 1h, 3h,
6h, 12h and 24h views. [Cooling details](docs/cooling-control.md)

An unlocked dashboard can remove any saved pool, including the last one. Removing
the last pool pauses mining until a pool is added and mining is resumed. Pool
changes do not clear an existing safety stop.

The unlocked **Network** page shows the connected Wi-Fi network, its IP address
and signal strength. Status refreshes every five seconds while the page is visible.
A brief notification names the network when a connection is confirmed; a connection
request alone is not reported as success.

There is no manual clock/voltage entry or customer tuning trial. Automatic
performance holds **430 mV**, commands both fans to **100%**, stays within the
unit's qualified clock ceiling and backs off for thermal pressure or excessive
invalid nonces. **Invalid nonces do not trigger a safety stop**, including in
default mode. Automatic mode tolerates isolated errors and learns a lower clock
ceiling when an elevated rate or a batch indicates insufficient margin. Temperature,
fan, telemetry, watchdog and clock/voltage protections remain active.
[Automatic performance policy](docs/automatic-performance.md)

Automatic performance can remain cooler than its limit once it reaches its
qualified clock ceiling; it commands 100% fans and does not heat the miner to
reach an exact temperature. The dashboard pauses polling in hidden tabs and
bounds retained data, requests and logs for extended operation.
[Dashboard endurance review](docs/dashboard-endurance.md)

The original maintenance page remains at `/legacy.html#/system`. Avoid firmware
uploads or factory resets as casual troubleshooting. The fork uses
`/usr/config/kdbox-xbt` and schema-6 backups, with complete schema-3/4/5 backups from
this fork also accepted. Evidence from one unit or finite test periods is not
certification or a guarantee for other hardware.

## Build from a source checkout

Use Python 3.12+ and stable **Go 1.27.3 or newer**, Git, and **Node.js 22+**
on your PATH. Linux race tests also need a host C compiler such as GCC.
Go and Node.js are needed on the build/test computer, not on the miner.
Commands below run from the repository root on Linux or Windows, using the
active Python environment. They do not contact a miner or upload artifacts.

```sh
python -m venv .venv
# Activate .venv using your shell's normal command, then:
python -m pip install -r requirements.txt
python tools/local.py test
```

If your default Go is older, select the required toolchain for this shell before
running the commands: `export GOTOOLCHAIN=go1.27.3` on Linux, or
`$env:GOTOOLCHAIN = 'go1.27.3'` in Windows PowerShell. Go downloads that toolchain
on first use, which requires Internet access. This does not change the system Go.

`test` builds the verified ARM payload, runs Go tests and vet, runs Go race tests
on Linux, runs the Python suite, and checks the dashboard and installer JavaScript files with
Node.js. It stops on the first failure. Run it on both Linux and Windows for the
same host coverage as the former GitHub Actions matrix; one local run checks
only its current host. Linux race checks run with CGO enabled; the ARM product
build always keeps CGO disabled.

For a build without tests, run `python tools/local.py build`. To launch the
installer after building, run `python -m flasher` (opens the local browser installer).

`tools/build.py` sets `GOOS=linux`, `GOARCH=arm`, `GOARM=7`, `CGO_ENABLED=0`,
generates the actual toolchain notices **before** embedding the UI, builds the
controller, embeds the source commit and clean/dirty state, and records hashes
in `build/payload.json`. Missing assets, mismatched
hashes, missing notices, or a wrong-architecture controller block installation.
Uncommitted builds show a `-dirty` suffix beside their commit. Builds without
Git provenance show `unversioned`; they cannot be packaged as a clean release.
The installer uses the binary's embedded revision even on computers without Git.
After editing deployed UI assets or scripts, rebuild: they are both embedded in
the controller and copied to the stock web directory.

The release build omits unused HTTP/2 support and tests with the same Go build
tag. The SSH installer stores the ARM controller on `/usr/config` and checks
available space there and on the root filesystem before writing.
[Controller size and storage](docs/controller-footprint.md)

To create all local artifacts from a clean, committed checkout:

```sh
python tools/local.py all
```

The product ZIP includes tracked application source, the controller, licenses, build
metadata, an environment inventory, and checksums. It excludes local virtual
environments and backups. New Go dependencies stop the packaging path until
reviewed. `tools/package.py` rejects source changes after the recorded build.
`all` runs the full test sequence, creates `dist/KDBoxXBT-<version>.zip`,
then creates `dist/kdbox-xbt-source.zip` and `dist/SHA256SUMS.txt` covering both ZIPs. Nothing is
published automatically. Commit intended source changes before running `all`;
builds and tests alone can run with local edits.

`python tools/local.py package` rebuilds and packages without running tests.
`python tools/local.py source` creates only the source archive and needs just
Python and Git. Like the former source workflow, that archive contains tracked
files from **committed HEAD**, with a `kdbox-xbt/` prefix; it excludes uncommitted
edits and generated build files. The lower-level `tools/build.py` and
`tools/package.py` commands remain available.

## Help and project documentation

For bugs, installation trouble or guidance, [open an issue in this repository](https://github.com/Spectre-Works/kdbox-xbt/issues).
Include the version and commit shown in the dashboard footer, computer OS, miner model and stock firmware,
the step that failed, and a redacted error message. Do not include passwords,
session URLs/tokens, private snapshots, payout details or raw configuration.
Limited DIY support does not include guaranteed recovery or response times.

- [1.0.1 release notes](docs/release-1.0.1.md)
- [1.0 release notes](docs/release-1.0.md)
- [First-install validation and recovery](docs/installation-handoff.md)
- [Platform setup and troubleshooting](docs/installer-hosts.md)
- [Dashboard endurance review](docs/dashboard-endurance.md)
- [Installer and live validation record](docs/installer-validation.md)
- [Architecture](docs/architecture.md)
- [Firmware compatibility and checksums](docs/firmware-compatibility.md)
- [Licensing and redistribution](docs/licensing.md)

This is a maintained derivative of the upstream application identified in [NOTICE](NOTICE).
Fork-specific issues belong here, not with the upstream author or Goldshell.
