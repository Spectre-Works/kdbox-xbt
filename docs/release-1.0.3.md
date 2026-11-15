# KD BOX XBT 1.0.3

Backups no longer require private folder or file permissions. Installation,
repair and updates can proceed on writable storage even when the filesystem
widens permissions or cannot apply POSIX permission modes. Permission-setting
failures are ignored without warnings or additional confirmation.

The installer still attempts restrictive modes where supported. Backup
completeness, file checksums, allowed recovery paths and miner identity checks
remain in place. Documentation now reflects this behavior.

## Download and update

Download `KDBoxXBT-1.0.3.zip` and `SHA256SUMS.txt` from this release, verify the
checksum, extract the entire ZIP and run the launcher for your platform. See the
[README](../README.md) and [host setup guide](installer-hosts.md).
`kdbox-xbt-source.zip` contains source only.

The supported installation base remains the original Goldshell KD BOX,
MCB_V5_4 / ARMv7, on stock Goldshell 2.2.1. Obtain vendor firmware separately using
the [firmware preparation guide](customer-firmware.md); none is bundled.

Existing installations with a different full source revision offer
**Back up & update KD BOX XBT**. Updating retains passwords, pools, network and
performance settings and existing stops. Mining pauses for the update and remains
paused for review. Factory reset remains a separate, explicitly confirmed action.

## Validation

Regression tests cover widened permissions, permissions changing during backup,
and denied or unsupported permission-setting operations. Backups complete in
those cases; corrupted transfers and incomplete backups still fail verification.

**Use at your own risk.** See the [README](../README.md) for installation limits,
recovery guidance and limited DIY support through GitHub Issues.
