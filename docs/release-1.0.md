# KD BOX XBT 1.0

Replacement 1.0 release with the corrected installer, Apache 2.0 licensing model,
reserved branding and the SPECTRE WORKS interface. Discard cached copies of the
withdrawn package and verify the checksums attached to this release.

Customer-installed SSH modification for the **original Goldshell KD BOX,
MCB_V5_4, ARMv7, running stock Goldshell 2.2.1**. Other KD BOX variants and stock
2.2.3/2.2.4 are not supported for new installations.

## Download and install

Read the [README](../README.md) before modifying your miner. Download
`KDBoxXBT-1.0.zip` and `SHA256SUMS.txt` from the
[1.0 release](https://github.com/Spectre-Works/kdbox-xbt/releases/tag/v1.0),
verify the ZIP checksum, extract the entire package, and open the launcher for
your OS. GitHub's automatic source downloads are not prepared installers.

The product ZIP includes the prebuilt ARM controller, editable installer/source,
licenses, checksums and a public-content audit. A separate
`kdbox-xbt-source.zip` contains the committed source only. `SHA256SUMS.txt` covers
both attached ZIPs. No Goldshell firmware, vendor mining binaries or signing
material are included. Obtain the stock 2.2.1 base separately from Goldshell.

Windows/macOS/Linux launchers prepare an isolated Python environment and open a
local browser wizard. Tk, Go and Git are not required to run the prepared
installer. The wizard provides bounded network discovery, owner/device and SSH
identity confirmation, exact stock-base checks, space/payload checks, a private
schema-6 snapshot and an explicit final install confirmation. It detects an
existing mod and offers dashboard/recovery access without reinstalling it.
Neither the dashboard nor SSH password is changed by installation.

## Installer and recovery refinements

- Official emblem, graphical wordmark, **REBUILT FOR BITCOIN.** tagline and website
  favicons, with local Chakra Petch fonts, dark/light themes and mobile layouts.
- The server starts before browser launch. A blocking desktop launcher cannot
  prevent HTTP replies; text browsers receive a clear JavaScript requirement.
- New installations select Bitcoin (XBT), persist the choice and verify stock API
  readback. Existing saved choices survive repair and restart. The dashboard
  reflects confirmed controller status rather than assuming a request succeeded.
- A working controller and startup/dashboard files identify an existing install.
  Leftovers after a stock reflash open a separate **Back up & repair** confirmation.
  Repair preserves saved settings, makes a new private backup and does not bypass
  safety stops or pending recovery journals.
- Schema-6 backups include the website favicons; complete schema-3/4/5 snapshots
  remain restorable. The private backup-folder override supports hosts where the
  normal home folder cannot retain private permissions.
- Apache 2.0 applies to SPECTRE WORKS contributions, upstream MIT attribution is
  retained, and modified redistributions must use their own product identity under
  the separate [branding policy](../BRANDING.md).

## Dashboard and operating defaults

- **75°C hottest-chip limit**, adjustable from 70–80°C; 30% normal fan minimum.
  Smoothing, hysteresis, warming prediction and a half-strength learned spike
  reserve capped at 3°C govern normal fan changes. Raw thermal, fan and watchdog
  protections remain active. This operating limit is separate from the critical
  thermal shutdown limits; short temperature excursions can occur.
- **Automatic performance off by default**, using 740 MHz / 430 mV. Optional
  automatic performance holds 430 mV and full fans within a per-unit qualified
  clock ceiling. There is no customer numeric clock/voltage entry or tuning trial.
- Invalid nonces do not trigger safety stops. Automatic performance tolerates
  isolated errors and reduces clock or rejects an increase when its bounded
  error budget is exceeded. See the [exact policy](automatic-performance.md).
- Live fan percentages/RPMs, temperatures, clock/voltage, accepted shares, pool
  state and Mining session duration. The session follows observed active mining
  independently of browser reloads. Hashrate history retains up to 24 hours and
  offers Auto, 1h, 3h, 6h, 12h and 24h ranges.
- The footer displays **1.0 and the source commit**, embedded in the controller
  at build time. Include both when reporting an issue.
- Diagnostics remain available during a safety stop where the stock service can
  supply them; live chip readings require a running engine. Recovery snapshots
  are private application backups, not complete firmware/bootloader images.

## Validation and limits

The project records live installation, startup, cooling and accepted-share checks
on one original KD BOX on stock 2.2.1. That unit's separate fixed-voltage test
retained 755 MHz / 430 mV over a finite twenty-minute zero-error run; this does not
raise the initial 740 MHz ceiling on other units or certify indefinite reliability.
[Observed firmware results](firmware-compatibility.md) ·
[Cooling results](cooling-control.md) · [Resource review](dashboard-endurance.md)

Linux setup and Chromium desktop/mobile flows were exercised end to end.
Windows/macOS have launchers and platform-specific setup guidance but were not
natively executed in those Linux validation sessions. First launch needs Internet
access for runtime/dependency downloads. Network isolation, host security policy
and unsupported architectures may require the documented manual setup.

**Use at your own risk.** Modification can interrupt mining, remove access,
damage hardware or brick a miner. Safeguards and snapshots cannot guarantee
recovery. SPECTRE WORKS provides limited DIY support via
[GitHub Issues](https://github.com/Spectre-Works/kdbox-xbt/issues), without guaranteed
recovery or response times. Keep backups, passwords and unredacted reports private.
This project is independent from Goldshell. The current source uses Apache 2.0
for SPECTRE WORKS contributions, retains upstream MIT notices, and separates
[brand-use terms](../BRANDING.md) from code licenses. See [licensing](licensing.md).
