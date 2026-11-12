# Why firmware 2.2.4 is rejected

Investigation date: 2026-11-10. Offline firmware analysis and SSH preflight on
one stock miner. Firmware installation and mining validation remain open.

## Finding

The rejection protects the installation/recovery access path. Goldshell KD BOX
2.2.4 retains the SSH daemon but removes the service-login branch used to start
it on 2.2.3. Its management daemon still stops SSH during startup. The older
`PUT /mcb/protect` endpoint is a no-op in both versions.

This is not evidence of a different board or hashing algorithm, or of an invalid
vendor image. It also does not establish that the XBT controller works on 2.2.4
if someone obtains shell access through a different mechanism. Merely adding
2.2.4 to the compatibility allowlist would leave the access problem unresolved.

## Image provenance and verification

Downloaded the original KD BOX images from the official Goldshell repository:

- [2.2.3 image](https://github.com/goldshellminer/firmware/blob/master/KDA-Miner/Goldshell-KDBox/firmware-Goldshell-KDBox-2.2.3.cpb)
- [2.2.4 image](https://github.com/goldshellminer/firmware/blob/master/KDA-Miner/Goldshell-KDBox/firmware-Goldshell-KDBox-2.2.4.cpb)

Both downloads matched the SHA-256 values already recorded in
[the compatibility table](firmware-compatibility.md). The internal model and
board files identify `Goldshell-KDBox` and `MCB_V5_4` in both images.

Both `firmware.ius.sig` signatures verified with OpenSSL SHA-256 using the
compatibility verification public key from the 2.2.3 image. The same public key
is present in 2.2.4. This is an image consistency/signature check against that
vendor-supplied key, not an independently obtained publisher trust anchor.
No signing keys were used, and no vendor firmware files are added to this repo.

## Observed differences

The extracted `/usr/sbin/sshd`, `/etc/init.d/S50sshd`, and `/etc/inittab` are
byte-identical between the two releases. The change is in `/usr/bin/minerd`,
the stock management/API program, rather than removal of the SSH executable.

| Check | 2.2.3 | 2.2.4 |
| --- | --- | --- |
| SSH daemon and startup script shipped | Yes | Yes, identical files |
| Management startup calls SSH stop | Yes | Yes |
| Service-login branch calls SSH start | Present | Removed |
| Service-login marker used by this installer | Present | Absent |
| `PUT /mcb/protect` handler | Returns without starting SSH | Same behavior |

Static disassembly details for reproducing the result (virtual addresses in
the exact extracted ARM Thumb executables):

- 2.2.3: the `/user/login` callback is at `0x1477a`. Its branch at
  `0x14874` compares the service-login marker, loads the SSH-start command
  through the literal at `0x149c0`, and calls the command wrapper at `0x1488e`.
  The command string is at `0x1e2df`.
- 2.2.4: the startup code loads the SSH-stop command at `0xb686` through the
  literal at `0xb7cc` and calls wrapper `0x1165c`; that wrapper calls `system`.
  The command string is at `0x1ddf3`. The SSH-start command and the service-login
  marker present in 2.2.3 are absent from this executable.
- The registered `/mcb/protect` callbacks are at `0xf258` in 2.2.3 and
  `0xf53c` in 2.2.4. Both consist of `movs r0, #0; bx lr`: they do not start SSH.

SHA-256 fingerprints of the extracted management executables:

```text
7b5f61162f7e3bb672c5233b10a4acb750cc7be85b2dfb762e0ef371d51bf733  2.2.3 /usr/bin/minerd
7911f1c7c9d34678165a0f2e45bcff45c10d89e977fedb2ff6ffd8e2e6aa3669  2.2.4 /usr/bin/minerd
```

Reproduction used Python's gzip/zlib decoding after the 512-byte CPB header,
tar inspection to obtain the IUS payload and signature, and the IUS rootfs
descriptor to locate the ext4 image. For these two exact files, the rootfs
starts at byte `9815552` and spans `33554432` bytes. Files were read with
`debugfs` without mounting or executing the firmware; ARM Thumb disassembly
used `llvm-objdump --triple=thumbv7-linux-gnueabi`.

These offline observations corroborate the
[upstream hardware report](https://github.com/Spectre-Works/kdbox-xbt/blob/5589e0604596722896266f7f0d565c1cc4484118/docs/firmware-compatibility.md),
which records unavailable stock SSH access on 2.2.4 and a successful manual
downgrade on its own test unit. That is not a downgrade test on our unit.

## Firmware signatures

The stock dashboard uses OpenSSL SHA-256 signature verification against the
vendor compatibility public key. An offline test flipped one byte inside the
2.2.4 IUS payload. Verification failed against the original signature, while
the unmodified image verified. The project uses a live SSH application install
on stock 2.2.1. [Stock firmware and the SSH installer](customer-firmware.md)

## Live check and implications for QC

The operator-designated miner reported `Goldshell-KDBox`, board `MCB_V5_4`,
firmware `2.1.3`. Its authenticated maintenance request opened SSH, and an SSH
session confirmed the identity and ARMv7 architecture. The live application
snapshot passed checksum validation. On this unit, the root filesystem had
about 3.4 MiB free and `/usr/config` had about 54 MiB free. Both passed the
application installer's capacity checks.

The ARM controller ran from temporary storage and passed startup preflight with
temporary configuration and no pending trial. The stock startup and dashboard
files and management process were verified after cleanup. The live system boots
from `/dev/mmcblk0p1` and has separate configuration and backup partitions.
The live updater and its launch script matched the pinned original 2.1.3
image. Goldshell documents updates through the dashboard for this model.
The presence of the local updater and a backup partition did not establish an
independent recovery path. No flash, persistent application install, or reboot
was performed. After testing, SSH returned to its initial closed state and the
dashboard still reported stock 2.1.3.
[Goldshell BOX Series Tutorial](https://goldshellhelp.zendesk.com/hc/en-us/articles/17608608063513-BOX-Series-Tutorial)

After the operator updated the unit, its status identified the same model and
board on 2.2.3. Ordinary dashboard authentication succeeded and the maintenance
request returned HTTP 200. Port 22 continued to refuse connections. After the
operator downgraded to stock 2.2.1, the same authenticated request opened port 22
and a root SSH session confirmed the pinned stock management build.
