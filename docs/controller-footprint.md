# Controller size and storage

Measured 2026-11-10 with stable Go 1.27.3, Linux ARMv7, CGO disabled. The
comparison below records the initial sizing work. Current builds print their
exact sizes.

## Cause

At the initial sizing revision there was no PNG in the embedded assets; the
only image was a 362-byte SVG. Current builds also include the reviewed
9,222-byte SPECTRE WORKS header PNG and local fonts. Their hashes and exact
current sizes are checked/printed by the build.
Before generation of the complete toolchain notices, all web files combined
were 32,531 bytes. With the actual Go toolchain notice inventory, the release
assets total 103,254 bytes, about 1.3% of the original release executable.

The executable size comes primarily from statically linked Go runtime and
standard-library code: HTTP client/server, TLS/cryptography, JSON/reflection and
other supporting code. The source application itself is small. The existing
build already strips debug/symbol information with `-s -w`.

## Changes and measured result

Both release builds and local Go checks now use `nethttpomithttp2`, omitting
unused HTTP/2 support. Dashboard requests and the stock loopback API use
HTTP/1.1. TLS, session randomness, authentication and safety behavior are not
replaced or weakened. The HTTP/2 build tag reduces the comparable stripped ARM
executable by 589,824 bytes (576 KiB).

| Artifact | Bytes | MiB |
| --- | ---: | ---: |
| Previous ARM executable, complete release notices | 7,798,944 | 7.44 |
| ARM executable with unused HTTP/2 omitted | 7,209,120 | 6.88 |
| Compressed firmware seed, complete payload | 2,968,414 | 2.83 |
| Firmware seed storage budget | 4,194,304 | 4.00 |

The live SSH installer stores the executable and scripts in
`/usr/config/kdbox-xbt`, on the separate configuration partition. It checks
space for the new controller plus staging headroom on that partition and for the
dashboard files plus headroom on the root filesystem. Stock 2.2.1 already mounts
its root filesystem writable for the dashboard and startup-file changes.

The table's compressed seed figures came from an earlier offline firmware
experiment. The current build produces `build/kdbox-xbt-controller` and records
payload hashes in `build/payload.json`. [Stock firmware and the SSH installer](customer-firmware.md)

The 32px and 48px SPECTRE WORKS favicons are copied from the website repository
and are reviewed by exact hash. The graphical installer wordmark stays in
`flasher/web/`, outside the embedded controller assets.
