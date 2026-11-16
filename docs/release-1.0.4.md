# KD BOX XBT 1.0.4

XBT startup now allows the fans to accelerate after stock calibration leaves
them at low output. Each fan has a fixed ten seconds from engine detection to
reach the unchanged 3,000 RPM floor. Once a fan is healthy, a subsequent stall
cannot reuse that allowance. Missing RPM readings and deadline expiry still
stop mining; thermal shutdown limits and the first-temperature deadline remain
unchanged.

Documentation now keeps reusable product guidance and validation procedures
in the repository; unit-specific testing and troubleshooting records stay private.

## Download and update

Download `KDBoxXBT-1.0.4.zip` and `SHA256SUMS.txt`, verify the checksum, extract
the entire ZIP and run the launcher for your platform. See the
[README](../README.md) and [host setup guide](installer-hosts.md).
`kdbox-xbt-source.zip` contains source only.

The supported base remains the original Goldshell KD BOX, MCB_V5_4 / ARMv7,
on stock Goldshell 2.2.1. Obtain vendor firmware separately; none is bundled.
Updates preserve passwords, pools, network/performance settings and existing
stops. Mining pauses for update and remains paused for review.

Validation covers slow initial spin-up, both fans, fixed deadlines, unreadable
RPM and loss of a previously healthy fan.

**Use at your own risk.** See the README for installation limits, recovery
guidance and limited DIY support through GitHub Issues.
