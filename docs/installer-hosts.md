# Installer host setup

Read the repository [README](../README.md) first. Use an extracted **prepared
product ZIP**, not GitHub's source archive. Keep the folder writable and launch
as your ordinary user. A modern desktop browser is needed for the local wizard.
The miner and computer must be reachable on the same trusted private network.

## Verify downloads

In the folder containing the downloaded ZIP, use the command for your OS and
compare its entire SHA-256 value with the matching line in `SHA256SUMS.txt`:

| OS | Product ZIP checksum command |
| --- | --- |
| Windows PowerShell | `Get-FileHash .\KDBoxXBT-1.0.1.zip -Algorithm SHA256` |
| macOS Terminal | `shasum -a 256 KDBoxXBT-1.0.1.zip` |
| Linux Terminal | `sha256sum KDBoxXBT-1.0.1.zip` |

The same commands can check Goldshell's separately downloaded CPB: replace the
ZIP filename with `firmware-Goldshell-KDBox-2.2.1.cpb` and compare with the pinned
SHA-256 in the README. This does not upload or distribute the firmware.

## What setup downloads

The launchers download **uv 0.13.7** from Astral's official GitHub release and
verify its archive against a pinned SHA-256 before executing it. uv then obtains
a managed Python 3.13 runtime and the packages in `requirements.txt` as needed.
Runtime files/cache stay under `.installer/`; libraries stay under `.venv/`.
The launchers do not modify system Python, global PATH, miner passwords or firmware.
These directories are excluded from public packages and source archives.

First setup requires HTTPS access to GitHub, Astral's Python distribution sources
and PyPI/package downloads. No account or miner data is sent to a project cloud.
Later launches reuse the environment and do not replace an installed Paramiko.
For a fresh setup, rename `.installer` and `.venv` while the installer is closed.
Never delete `~/KDBoxXBT/backups/` as part of resetting the host environment.

## Windows

Use Windows 10/11 x64 or ARM64 and double-click `install.cmd` after extraction.
The launcher uses built-in Windows PowerShell with **RemoteSigned for that process
only**, not a persistent policy change. If Windows marks downloaded files as blocked,
verify the trusted ZIP first, use **Properties → Unblock → Apply** on the ZIP,
then extract again. Do not disable antivirus or organizational policies.

If PowerShell is disabled or policy-enforced, ask your administrator or use the
manual Python method below; this project does not configure a managed computer.
If a security prompt or firewall prompt appears, verify the files and permit only
the private network access you intend. Do not grant public-network access.
[Microsoft: downloaded file blocking](https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.utility/unblock-file)
· [Execution policies](https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.core/about/about_execution_policies)

## macOS

Double-click `install.command`. If Finder opens an editor or reports permission
denied, open Terminal in the extracted folder and run `sh install.sh` instead.
The prepared ZIP stores the command file as executable; some extraction tools
lose that permission. This source launcher is not a signed/notarized native Mac
application. Respect macOS security prompts and organizational restrictions.
The wizard uses your normal browser; Tk/XQuartz are not needed.

## Linux

Run `sh install.sh` in the extracted folder. The automatic launcher supports
64-bit x86_64 and aarch64 systems with glibc and needs `curl`, `tar`, and either
`sha256sum` or `shasum`. On Debian/Ubuntu, your normal package manager can provide
missing utilities: `sudo apt install curl tar coreutils ca-certificates`.
Other distributions should use their equivalent packages.

Minimal/headless systems, Alpine/musl, 32-bit hosts or systems too old for Astral's
managed Python builds need the manual method or a supported desktop. The installer
does not reconfigure libc, install a desktop, or change system security policy.
[uv platform/runtime support](https://docs.astral.sh/uv/reference/platforms/)

## Manual Python method

If you already have **Python 3.12+**, you can skip the bootstrap entirely. Install
Python from [python.org](https://www.python.org/downloads/) or your platform's
package manager. On Linux you may also need your distribution's `python3-venv`
package. Tcl/Tk is not required. From the extracted product folder:

```sh
python -m venv .venv
# Windows:
.venv\Scripts\python.exe -m pip install -r requirements.txt
.venv\Scripts\python.exe -m flasher
# macOS / Linux (use python3 above if needed):
.venv/bin/python -m pip install -r requirements.txt
.venv/bin/python -m flasher
```

Use only the commands for your OS. Prepared packages need neither Go nor Node.
Source checkouts first need the maintainer build procedure in the README.
[Python installation guide](https://docs.python.org/3/using/index.html)
· [uv installation/runtime guide](https://docs.astral.sh/uv/guides/install-python/)

## Discovery and recovery problems

Discovery uses read-only HTTP `GET /mcb/status` on up to four directly connected
private IPv4 subnets. Each is limited to the computer's local /24 (or smaller),
with at most 1,024 addresses and 32 concurrent probes. It does not enumerate your
entire /16 or corporate network. On a larger LAN, use **Look up an IP address**
with the miner's address from the router. IPv6-only discovery is not supported.

Turn off an unrelated VPN or switch from guest Wi-Fi to the trusted LAN if that
is appropriate for your network. Do not disable firewalls globally. No result can
mean the miner is booting, another model, unreachable or network-isolated. A
found miner on another stock version is shown but cannot be installed until you
separately prepare the supported 2.2.1 base.

The local wizard binds only to `127.0.0.1` on a random port and needs its private
session URL/token. If the session is unavailable, reopen the complete URL
from the launcher window. Keep that window open during work; use **Close installer**
after completion. Never share the session URL or close the launcher during writes.

An existing installation is detected and is not overwritten by this first-install
wizard. Open its dashboard or use its recovery backups. For backups copied from
another computer, place the **complete original snapshot directory** beneath
`~/KDBoxXBT/backups/` before starting the installer. Device, firmware and hash
checks still apply. No arbitrary local paths can be supplied through the browser.

If writes already began and an error occurs, keep the backup path, leave the
miner powered, inspect its dashboard and consult [recovery](installation-handoff.md).
For help, [open a GitHub issue](https://github.com/Spectre-Works/kdbox-xbt/issues)
with a redacted error and your OS/version. Never attach private backups or credentials.

## Private backup folder

The default is `~/KDBoxXBT/backups/`. If a shared/synced home folder or host policy
prevents private backups, choose an absolute path on a private local filesystem
before launching. For example, on Linux/macOS:

```sh
KDBOX_XBT_BACKUP_DIR=/absolute/private/backups sh install.sh
```

In Windows PowerShell, set `$env:KDBOX_XBT_BACKUP_DIR = 'C:\Users\YOUR-NAME\KDBox-private-backups'`
then run `./install.ps1`. Set the same path on later launches to see those backups
in the recovery list. Keep it outside source checkouts and shared/synced folders;
on Windows verify user ACLs. On POSIX hosts the installer checks snapshot
permissions both before collection and again before application writes. A host can
change permissions later, so retain backups on storage you control.

## Browser says “contacted” but never loads

The launcher starts the local HTTP server before attempting to open a browser.
Browser detection and opening run separately, so a blocking desktop launcher cannot
prevent the server from replying. Known text browsers such as w3m and lynx are not
started: the guided UI requires JavaScript. Open the printed local URL in a modern
browser on the same computer. Keep the launcher terminal open, and keep the URL
private because its fragment contains the installer session token.

On a terminal-only or remote session without a graphical browser, this message is
an instruction to open a compatible browser, not a firmware/network failure. Do
not enable network-wide access to the local installer or disable its token checks.
The supported shell entry point is `sh install.sh`.
