# KD BOX XBT Mod

**SPECTRE WORKS · original Goldshell KD BOX · experimental preview**

Enable XBT (Bitcoin BLAKE2b) mining on compatible original KD BOX miners,
with **KD BOX Control**, a replacement dashboard for monitoring, pool management,
conservative tuning, cooling controls, and recovery. The desktop tool is
**KD BOX XBT Installer**.

This is a maintained derivative of [Paperclip Flasher](https://github.com/connorslab/paperclip-flasher),
not a complete firmware image. It installs files over SSH and uses the mining
engine already on the device. It does not change the ASIC into a different
hashing algorithm, replace the bootloader, or upgrade/downgrade stock firmware.
KDA selection remains available. No vendor firmware or signing keys are included.

## Compatibility and evidence

Only **original Goldshell KD BOX**, **MCB_V5_4**, **ARMv7**, and firmware explicitly
listed in [`controller/compatibility.json`](controller/compatibility.json) are
accepted. **KD BOX II, other boards/models, and firmware 2.2.4 are not supported.**
Do not bypass identification checks to try them.

The [compatibility table](docs/firmware-compatibility.md) preserves upstream
reports from one test unit. Those are **not tests of this SPECTRE WORKS fork**.
The fork is a hardware-unvalidated preview until the
[first-install checklist](docs/installation-handoff.md) is completed on a real device.
Automated test success does not demonstrate cooling, mining, or recovery on hardware.

## Risk and responsibility

**Installation can interrupt mining, remove access, brick a miner, or damage
hardware.** Startup-file changes and tuning have risks even without flashing a
firmware image. Software thermal stops, watchdogs, and rollback cannot guarantee
recovery or prevent every failure. A frozen or unpowered miner cannot execute
recovery until it runs again; some failures require specialist recovery tools.

Use a trusted LAN and a physically supervised miner with reliable power and
cooling. Never expose ports 22, 80, or 8088 to the Internet. Management HTTP is not
encrypted. The software is supplied without warranty under [LICENSE](LICENSE).
These software terms do not replace any separate seller warranty or applicable law.

## Install a prepared package

The default artifact is an **editable Python installer plus a prebuilt ARM
controller**, not a frozen `.exe`. It intentionally does **not** bundle Python,
Tcl/Tk, Paramiko, or their dependencies. This keeps separately licensed libraries
replaceable and avoids distributing an unaudited one-file dependency bundle.
See [licensing and redistribution](docs/licensing.md).

1. Use a successful **Test and build KD BOX XBT preview** workflow from
   [this fork's Actions](https://github.com/Spectre-Works/kdbox-xbt/actions), not
   an upstream Paperclip executable. Download the `KDBoxXBT-preview-…` artifact.
   Extract its product ZIP and check `SHA256SUMS.txt` against the exact downloaded
   ZIP before extracting it. Checksums detect corruption, not an untrusted publisher.
2. Install **Python 3.12+ with Tcl/Tk** on Windows or Linux. Linux may require
   distribution packages such as `python3-venv` and `python3-tk`.
3. Open `install.cmd` on Windows, or run `sh install.sh` on Linux. The first launch
   creates a local `.venv` and uses pip to obtain Paramiko and dependencies.
   That first setup needs Internet access; subsequent launches reuse the environment.
   No miner is contacted until you enter its details and start an operation.
4. Follow [the installation handoff](docs/installation-handoff.md). Have the miner's
   IP, its actual SSH root password, and its separate dashboard password ready.
   Review the modification warning and verify the SSH fingerprint when prompted.
5. Choose **Back up & install KD BOX XBT**, then open `http://MINER-IP/`.
   Unlock Mining, verify the pool and payout/worker, and select **blake2b(XBT)**.
   Installation preserves existing pool and algorithm settings; it does not silently
   choose a pool, donation address, or algorithm for you.

The normal GUI warns about modifications before connecting; the SSH identity is
confirmed during connection. No prompt should be accepted blindly by an automation.

New private recovery snapshots live under `~/KDBoxXBT/backups/`. The restore
picker still discovers the old `~/PaperclipFlasher/backups/` location. Do not
upload snapshots to GitHub or share them with support: they can contain passwords,
Wi-Fi secrets, pool credentials, and identifying device information. On Windows,
verify that the containing user profile/folder has appropriate ACLs.

## Build from a source checkout

Use Python 3.12+ and stable **Go 1.27.3 or newer**. Go is needed on the build
computer, not on the miner. Commands below run locally and do not contact a miner.

```sh
python -m venv .venv
# Activate .venv using your shell's normal command, then:
python -m pip install -r requirements.txt
python tools/build.py
go test ./...
go vet ./...
python -m unittest discover -s tests -v
python -m flasher
```

`tools/build.py` sets `GOOS=linux`, `GOARCH=arm`, `GOARM=7`, `CGO_ENABLED=0`,
generates the actual toolchain notices **before** embedding the UI, builds the
controller, and records hashes in `build/payload.json`. Missing assets, mismatched
hashes, missing notices, or a wrong-architecture controller block installation.
After editing deployed UI assets or scripts, rebuild: they are both embedded in
the controller and copied to the stock web directory.

To publish from a clean, committed checkout:

```sh
python tools/build.py
python tools/package.py
```

The ZIP includes tracked application source, the controller, licenses, build
metadata, an environment inventory, and checksums. It excludes local virtual
environments and backups. New Go dependencies stop the packaging path until
reviewed. `tools/package.py` rejects source changes after the recorded build.

## Dashboard and safeguards

Overview, Mining, Network, System, and Diagnostics remain available. The original
maintenance interface remains at `legacy.html`; it bypasses the new control flow.
Do not use stock firmware uploads or factory reset as casual troubleshooting.

Inherited tuning limits remain 500–800 MHz, 400–430 mV, and 80–100% fan output.
Changes use a three-minute trial and require explicit confirmation after healthy
mining and a new accepted share. Clock increases are limited to 25 MHz and voltage
changes to 5 mV per trial. Thermal shutdown, watchdogs, pending-trial recovery,
network rollback, and firmware-specific limits are retained, not weakened for
branding. **Firmware 2.0.6 has tighter thermal limits.** Read the compatibility
and recovery documentation before tuning.

Internal `/usr/config/paperclip` paths, startup names, and the authentication
cookie are deliberately unchanged to preserve upgrade/recovery compatibility.
Those are implementation identifiers, not promotional branding. A namespace
migration would be a separate hardware-tested change.

## Documentation and support

- [First installation and local-LLM handoff](docs/installation-handoff.md)
- [Architecture, upgrade and recovery](docs/architecture.md)
- [Upstream firmware evidence](docs/firmware-compatibility.md)
- [Licensing and redistribution](docs/licensing.md)

SPECTRE WORKS maintains this derivative. Do not send fork-specific support
requests to the upstream author as though they shipped it. Review and redact
logs before reporting a problem; Diagnostics may expose sensitive configuration.
