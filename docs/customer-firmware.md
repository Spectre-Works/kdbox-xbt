# Stock firmware and the SSH installer

Use **stock Goldshell 2.2.1** on the original **KD BOX / MCB_V5_4 / ARMv7**.
Get the original image directly from Goldshell if you need to prepare or recover a
miner. Check the published CPB checksum and follow the vendor’s firmware procedure
separately from the application install. Unlock the stock dashboard, open
**System**, select the separately downloaded `.cpb` in the firmware-update field,
choose **Update**, and wait for completion/restart without interrupting power.
[Goldshell firmware-update guide](https://goldshellhelp.zendesk.com/hc/en-us/articles/16805936980633-Upgrade-Miner-Firmware)
[Goldshell 2.2.1](https://github.com/goldshellminer/firmware/blob/master/KDA-Miner/Goldshell-KDBox/firmware-Goldshell-KDBox-2.2.1.cpb) · [Compatibility and checksums](firmware-compatibility.md)

**Avoid 2.2.4 for this mod.** Stock 2.2.3 and 2.2.4 stop SSH when the management
process starts, and the ordinary dashboard maintenance request no longer starts
it. Version 2.2.4 also removes the 2.2.3 service-login branch and changes the
kernel, mining programs, and fan controller. [Firmware comparison](firmware-2.2.4-investigation.md)

## Install on the running miner

1. Launch the guided installer and authorize read-only discovery on your own
   private LAN. Select and confirm your miner, then enter its current dashboard
   password. Leave the advanced SSH field blank for the stock 2.2.1 default, or
   supply your changed root password. If SSH is
   closed, the installer checks the reported model, board and 2.2.1 version,
   signs in through the ordinary stock dashboard and starts SSH. This initial
   request uses unencrypted HTTP on the trusted LAN.
2. Confirm ownership, recovery access, stable power and cooling. Verify the
   miner’s SSH fingerprint when connecting.
3. Select **Back up & install KD BOX XBT**. Over SSH, the installer confirms the
   model, board, architecture, reported version, local version file, and stock
   `minerd` SHA-256. It authenticates using the current dashboard password and
   keeps both original passwords. It saves a private application snapshot before
   writing the controller, dashboard, and startup configuration.
4. Verify boot, cooling, accepted XBT shares, restart persistence, and recovery.
   [Installation handoff](installation-handoff.md)

The installer uses the firmware already on the device. Stock firmware images
and independent firmware recovery remain separate from the application snapshot.
Release archives contain SPECTRE WORKS application code, assets, build metadata,
and open-source notices. The release audit checks both public ZIPs for vendor
firmware, proprietary binaries, signing material, private files, and binary deltas.
[Licensing and redistribution](licensing.md)
