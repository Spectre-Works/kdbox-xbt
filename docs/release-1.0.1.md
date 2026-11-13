# KD BOX XBT 1.0.1

This update makes the Wi-Fi connection state clear in KD BOX Control and adds
confirmed connection notifications, with guided setup that distinguishes fresh
stock miners from genuine incomplete installations. The installer and dashboard
report **1.0.1**.

## Wi-Fi improvements

- The Network page distinguishes connected, disconnected, disabled and unavailable
  Wi-Fi states. A connected miner shows its network name, Wi-Fi IP address and
  signal strength, with the active network marked in the network list.
- Connection status refreshes every five seconds while the unlocked Network page
  is visible. Requests do not overlap, and background polling pauses when the
  dashboard is hidden. Unchanged network lists are retained instead of rebuilt.
- A short-lived notification names the network when the native firmware confirms
  a connection. Requesting a connection alone does not report success, and repeated
  status reads do not repeat the notification for the same connection.
- Disconnect uses the current connected network. Forget network can use a selected
  network or the current connection when the name field is empty.
- Unavailable status hides stale connection details; locking controls clears the
  private network view. Saved Wi-Fi passwords are stripped from the native status
  response before it reaches the dashboard.

## Installer detection

Fresh stock dashboards follow **Ready to install**. A stock `index.html`, generic
assets or a favicon do not establish a previous installation. Repair requires
mod-specific runtime, startup or branding evidence; complete file and controller
checks still identify an already-installed miner. Genuine remnants retain the
separate repair confirmation, private backup and saved-settings protections.

## Download and installation

Read the [README](../README.md), then download `KDBoxXBT-1.0.1.zip` and
`SHA256SUMS.txt` from the [1.0.1 release](https://github.com/Spectre-Works/kdbox-xbt/releases/tag/v1.0.1).
Verify the checksum, extract the entire ZIP, and open the launcher for your OS.
`kdbox-xbt-source.zip` contains source only; GitHub's automatic source downloads
are not prepared installers.

Installation requires the **original Goldshell KD BOX, MCB_V5_4, ARMv7, running
stock Goldshell 2.2.1**. Obtain that firmware directly from Goldshell using the
[firmware preparation guide](customer-firmware.md). No vendor firmware is bundled
or installed by this tooling. Other KD BOX models and stock 2.2.3/2.2.4 are not
supported for new installations.

The installer detects a working existing installation and offers dashboard and
recovery access; it does not automatically overwrite that installation. These
release downloads do not add an in-place update wizard.

## Validation and operating limits

The Wi-Fi implementation was checked on one original KD BOX on stock 2.2.1, on
both dashboard ports and desktop/mobile layouts. Confirmed network name, IP,
signal, notification expiry and lock privacy were verified against that unit.
Connection/disconnection transitions and read failures were tested with simulated
responses to avoid interrupting its live Wi-Fi connection. A supervised application
deployment and resume returned all 16 chip temperatures and new accepted shares;
passwords, saved network/mining settings and thermal safeguards were preserved.
Fresh-install detection was also verified read-only on a never-modified stock
2.2.1 miner with only the stock `index.html` present. Regression tests cover shared
stock assets, genuine remnants, working controllers and the new-install wizard
review. These are finite observations, not certification of other hardware or
indefinite operation.

Cooling defaults remain a 75°C hottest-chip limit and 30% normal fan minimum.
Automatic performance remains off by default, with 740 MHz / 430 mV defaults.
The critical thermal, fan and watchdog protections remain active.

Prepared launchers support Windows, macOS and Linux. Native Windows/macOS execution
was not part of the Linux validation session.

**Use at your own risk.** Modification can interrupt mining, remove access, damage
hardware or brick a miner. Backups and safeguards cannot guarantee recovery.
SPECTRE WORKS provides limited DIY support through [GitHub Issues](https://github.com/Spectre-Works/kdbox-xbt/issues),
without guaranteed recovery or response times. This project is independent from
Goldshell. See [licensing](licensing.md) and the separate [branding policy](../BRANDING.md).
