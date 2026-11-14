# KD BOX XBT 1.0.2

Factory reset now clears owner personalization while keeping the mod installed.
Recovery from the original stock reset is clearer, and a normal no-pool setup
pause starts mining automatically after a valid pool is saved. The installer and
dashboard report **1.0.2**, with the controller's source revision in the footer.

## Factory reset — keep the mod

In **System**, choose **Factory reset — keep the mod** and type **FACTORY RESET**.
This deliberately removes pools, saved Wi-Fi networks, static IP/DNS, local cloud
credentials, device labels, clock qualifications/tuning preferences, mining history
and recognized owner credential/cache files. It removes SSH authorized keys,
rotates the native dashboard signing keys and invalidates mod sessions.

The dashboard password returns to **123456789** and the SSH root password to
**!@#$%123**. The mod stays installed. New defaults are Bitcoin (XBT), 740 MHz /
430 mV, a 75°C hottest-chip limit and Automatic performance off.

**Wi-Fi disconnects and the miner reboots.** Have Ethernet and router/installer
discovery available before confirming; the miner's IP may change. Genuine or
unknown safety faults remain latched for inspection. Benign setup/recovery/update
pauses become fresh owner setup after the confirmed reset completes.

The reset uses a persistent journal. Failed or ambiguous erases require an
explicit retry, and completion requires a verified new boot. It is managed-state
cleanup, not secure flash erasure, a full OS reimage or a remote cloud-account
unlink. Copies in off-device backups remain under owner control.

## Recovery and setup pauses

- The dashboard shows recovery instructions when its page loads but its controller
  is unavailable, and warns against using the original stock factory reset.
- A small persistent launcher outside the resettable settings partition maintains
  SSH access and full cooling and holds mining when application files are missing.
  It does not restore old credentials or start mining. Use the installer's repair
  flow; a firmware reflash can still overwrite the launcher.
- Repair after erased private settings restores defaults and verifies a persistent
  recovery pause before reporting success. A completely absent pool list is
  initialized empty; surviving settings and genuine stops are retained.
- A known **pool-setup pause** resolves automatically after a valid saved pool and
  the managed configuration are verified. It is no longer presented as a safety
  fault requiring a manual clear. Thermal, fan, watchdog, unknown, recovery and
  software-update inspection stops remain latched. A failed automatic start becomes
  an inspection-required fault rather than a repeated retry loop.
- Versioned, allowlisted recovery snapshots now include the persistent launcher
  in schema 7. Complete schema 3–6 snapshots remain supported.

## Download and update

Read the [README](../README.md). Download `KDBoxXBT-1.0.2.zip` and
`SHA256SUMS.txt`, verify the checksum, extract the entire ZIP and run the launcher
for your platform. The product ZIP contains the ARM controller and installer;
`kdbox-xbt-source.zip` and GitHub's automatic source downloads contain source only.

The installer requires the **original Goldshell KD BOX, MCB_V5_4, ARMv7, on stock
Goldshell 2.2.1**. Obtain that base directly from Goldshell using the
[firmware preparation guide](customer-firmware.md). No vendor firmware is bundled
or installed. Other models and stock 2.2.3/2.2.4 are not supported installation bases.

An existing installation is compared with the package by full Git revision,
version, modified-build flag and controller SHA-256. Different builds offer
**Back up & update KD BOX XBT**. Updating application software preserves passwords,
network/mining settings and existing stops; it leaves mining paused for review.
Factory reset is a separate, explicitly confirmed destructive action.

## Validation and limits

A full reset was tested on one original KD BOX / MCB_V5_4 on stock 2.2.1 after
seeding non-default web/SSH passwords, an accepted SSH key, a test pool, inactive
Wi-Fi credentials, persistent static-IP/DNS settings, cloud/device-label markers,
tuning preferences and history. After reboot, factory credentials worked, test
credentials and the old mod session were rejected, personalization was absent,
and defaults were restored. The mod and recovery guard survived; both dashboard
ports, desktop/mobile setup and read-only installer detection passed. The unit
was left locked, without pools or hashing, ready for fresh setup.

The live test exposed stock minerd restoring its private signing key from its
startup template. Reset now rotates both the templates and runtime keys. The
corrected run verified old JWT signatures against the old key and their rejection
by the new key, plus valid signatures for fresh login tokens. Automated fixtures
also cover retained faults, partial failures and reboot/journal behavior. These
are finite observations, not certification of other hardware or indefinite operation.
Native Windows/macOS execution was not part of the Linux validation session.

**Use at your own risk.** Modification and reset can interrupt mining, remove
access, damage hardware or brick a miner; backups and safeguards cannot guarantee
recovery. SPECTRE WORKS offers limited DIY support through
[GitHub Issues](https://github.com/Spectre-Works/kdbox-xbt/issues), without guaranteed
recovery or response times. This independent project is not endorsed or supported
by Goldshell. See [licensing](licensing.md) and the separate
[branding policy](../BRANDING.md).
