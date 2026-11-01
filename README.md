# Paperclip Flasher

A local installer and replacement dashboard for the original Goldshell KDBox,
with BLAKE2b mining, cooling controls, and recoverable tuning trials.

**Experimental preview.** The adapter supports the original KDBox, board
**MCB_V5_4**, ARMv7. The **0.1.1 prerelease supports 11 tested firmware versions**; see the
[firmware compatibility table](docs/firmware-compatibility.md) for additional
supported versions and test results. Firmware 2.2.4 and unsupported hardware are rejected.
This installs reversible files over SSH; it does not flash a complete firmware
image, downgrade a device, or modify its bootloader. The project contains no
vendor firmware or vendor signing keys. Your SSH and dashboard passwords are entered locally.

## Risk and responsibility

**Use this software at your own risk.** Firmware-related changes can brick a
device, including changes to startup files and mining software that do not flash
a full firmware image. Installation, interrupted updates, incompatible settings,
and tuning can cause loss of access, lost mining income, or permanent hardware
damage. Recovery may require specialist tools or may not be possible.

Back up your miner before making changes. Automatic rollback and thermal checks
reduce risk but cannot guarantee recovery or prevent every failure. This software
is provided without warranty; see [LICENSE](LICENSE).

## 0.1.2 candidate

The next version adds bounded retries for SSH greetings and read-only device
checks, explicit connection progress, verified backup/file transfers, and a
startup check before reporting installation success. Wrong passwords and changed
SSH identities stop immediately. Installation writes, login requests and reboot
commands are never automatically replayed after a lost connection.

The published download below remains 0.1.1 until candidate validation is complete.

## Use it

1. Put your computer and miner on the same trusted LAN.
2. Download the Windows or Linux build from the [0.1.1 prerelease](https://github.com/connorslab/paperclip-flasher/releases/tag/v0.1.1),
   extract the archive, and open `PaperclipFlasher`. Archive checksums are included
   in `SHA256SUMS.txt`. Linux may require marking the file executable.
3. Enter the miner's IP, SSH password, and dashboard password.
4. Confirm the device's SSH fingerprint. Choose **Back up & install Paperclip**.
5. Open the miner's normal address, `http://MINER-IP/`.
6. Unlock **Mining**, add your pool URL and payout address/worker, then select
   the matching algorithm. For XBT mining, select **blake2b(XBT)**.

Installation preserves existing pools and the selected algorithm. Pool details
are configured in the dashboard, not in the installer.

### Stock SSH credentials

Use username **`root`** and the password for your installed stock firmware:

| Firmware | SSH password |
| --- | --- |
| 2.0.5–2.0.8 | `19283746` |
| 2.0.9–2.2.1 | `!@#$%123` |
| 2.2.3–2.2.4 | `yuiop^&*` |

These passwords match the available official firmware images. See the
[compatibility table](docs/firmware-compatibility.md) for versions that have also
passed installation and mining tests. A known password alone does not mean the
installer supports that firmware.

Enter it in **SSH root password**. The dashboard password is separate. If the
SSH password has been changed, use the current one.

The installer makes a private recovery backup under `~/PaperclipFlasher/backups` before changing anything. It does
not try to upgrade unsupported firmware or recover unknown passwords. If SSH is
off, the installer verifies the supported model and enables the existing SSH
server using the dashboard password. This stock API uses unencrypted HTTP; run
the installer only on a trusted LAN. Installation file transfers use SSH.

## Dashboard

- **Overview:** live and average hashrate, history, pool shares, temperatures,
  and hardware-error percentage with the actual nonce denominator.
- **Mining:** saved pools and active status remain visible while locked; unlock to edit pools and priority, `blake2b(XBT)` / `blake2s(KDA)`,
  fixed or temperature-based fan control, clock and voltage trials.
- **Network:** DHCP or static IPv4, gateway, automatic or manual DNS, and Wi-Fi
  controls when an adapter is present. Unconfirmed network changes roll back.
- **System:** device details, identification LED, dashboard password, restart,
  and optional native cloud/lighting controls when supported.
- **Diagnostics:** per-chip temperatures and errors, mining/kernel/system logs,
  process and memory information, with downloads.

The original page remains available as `legacy.html` for advanced maintenance.
Unverified stock firmware uploads are deliberately excluded from the new UI:
some versions remove SSH access. Factory reset/legacy maintenance bypasses the
Paperclip control flow and can remove settings; use the recovery guide first.

## Tuning safeguards

| Setting | Initial supported limits |
| --- | --- |
| Clock | 500–800 MHz; 5 MHz steps; increases limited to 25 MHz per trial |
| Core voltage | 400–430 mV; 5 mV changes per trial; no above-stock voltage |
| Fan output | 80–100%; fixed speed or a temperature curve |
| Trial | 3 minutes; explicit confirmation after healthy mining and a new accepted share |

The controller records the previous settings before applying a trial, reloads
the mining engine, and checks its **live** clock and voltage. Expiry, unstable
trial telemetry, or a controller restart restores the previous settings. Startup
restores the hardware parameters on disk before launching the mining supervisor,
then completes recovery through the native API. A separate process
watchdog covers a stuck controller. A whole-system freeze may still require a
power cycle; the pending trial is recovered at the next boot.

Thermal shutdown remains enabled. Missing temperatures, low fan RPM, average
temperature at 84°C, or a reported chip temperature at 90°C stops mining and
retains management access. Trials also roll back at lower temperature limits or
an excessive invalid-nonce rate. These are conservative software limits, not a
guarantee against hardware failure. Firmware 2.0.6 uses tighter shutdown limits
(80°C average / 85°C chip). Device-specific cooling still matters.

## Build from source

Install Python 3.12+, Tk support, and Go 1.27.3. On Linux, Tk may be supplied by
your distribution's `python3-tk` package.

```text
python -m pip install -r requirements.txt
go test ./...
go vet ./...
python -m unittest discover -s tests -v
```

Build the on-device controller with `GOOS=linux`, `GOARCH=arm`, `GOARM=7`,
`CGO_ENABLED=0`, then:

```text
go build -trimpath -ldflags="-s -w" -o build/paperclip-controller ./controller
python -m flasher
```

On PowerShell set those variables with `$env:GOOS='linux'` (and likewise for the
other variables). On Linux prefix the build command with the four assignments.
The workflow builds Windows and Linux installer previews without publishing a
release. See [architecture and recovery](docs/architecture.md).

## Connection troubleshooting

- **SSH greeting timed out:** verify the IP opens the miner dashboard and let it
  finish booting. The 0.1.2 candidate makes up to four connection attempts with a
  30-second greeting timeout. If those fail, power-cycle the miner once and retry.
- **Password rejected:** enter the SSH root password for the installed firmware,
  not the dashboard password. Authentication failures are not retried.
- **SSH unavailable:** provide the dashboard password to enable SSH on a supported
  model. A slow greeting alone does not trigger changes to SSH settings.
- **Installation interrupted:** keep the miner powered on. The installer shows
  the recovery backup location; inspect the dashboard before choosing recovery.
  An interrupted operation may have applied some changes even if no reply arrived.

Do not upgrade stock firmware to solve a connection error. Never send passwords,
backup archives, or private pool credentials with a support request.
