# KD BOX XBT 1.0.2

Fixes an installer detection error that sent a never-modified stock miner to
**Repair previous installation** instead of **Ready to install**.

## Installer fix

The stock Goldshell dashboard already contains `index.html`. The detector was
incorrectly treating the presence of any dashboard asset as evidence of an old
mod. It now requires a mod-specific runtime directory, startup entry, dashboard
branding or branded asset before offering repair. Shared stock filenames and
generic favicons do not establish a previous installation.

Full file and controller checks still distinguish working installations from
genuine incomplete installations. Repair confirmation, recovery backups and
safety-stop handling are preserved.

## Download and use

Read the [README](../README.md), then download `KDBoxXBT-1.0.2.zip` and
`SHA256SUMS.txt` from the [1.0.2 release](https://github.com/Spectre-Works/kdbox-xbt/releases/tag/v1.0.2).
Verify the checksum and extract the entire package. Close an older installer and
start the launcher in the new package; an already-running installer keeps its
loaded code. On a fresh stock miner, verification should reach **Ready to install**.

The prepared installer includes the ARM controller; `kdbox-xbt-source.zip` and
GitHub's automatic source downloads are source only. No Goldshell firmware or
vendor mining binaries are bundled. Installation still requires the **original
Goldshell KD BOX, MCB_V5_4, ARMv7, on stock Goldshell 2.2.1**. Obtain that firmware
separately using the [preparation guide](customer-firmware.md).

## Validation and limits

Read-only checks reproduced the false repair classification on one never-modified
stock 2.2.1 miner: only its stock `index.html` was present, with no mod runtime
directory or startup hook. The corrected detector classified that same unit as
a new installation without writing files or changing its configuration.
Regression tests cover stock/shared assets, real mod remnants, working controllers,
calibration, retained safety stops and the wizard's new-install review path.

This patch does not change the controller's Wi-Fi display, mining, cooling or
thermal policy from 1.0.1. It does not add an in-place update wizard. Native
Windows/macOS execution was not part of the Linux validation session.

**Use at your own risk.** Modification can interrupt mining, remove access, damage
hardware or brick a miner. Safeguards and backups cannot guarantee recovery.
SPECTRE WORKS provides limited DIY support through [GitHub Issues](https://github.com/Spectre-Works/kdbox-xbt/issues).
This project is independent from Goldshell. See [licensing](licensing.md) and
the separate [branding policy](../BRANDING.md).
