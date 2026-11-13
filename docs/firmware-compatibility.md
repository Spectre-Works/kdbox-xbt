# Firmware compatibility

**SPECTRE WORKS installation base: stock 2.2.1** on the original KD BOX /
MCB_V5_4 / ARMv7. The installer can start SSH over the LAN and verifies
the stock management build over SSH. **Avoid 2.2.4.** Its management process
stops SSH at startup and the 2.2.3 SSH enable mechanism was removed. Its mining
and fan-control binaries also changed. [Stock firmware and SSH](customer-firmware.md)

## Current installation policy

New installs require the original KD BOX / MCB_V5_4 / ARMv7 and stock 2.2.1.
Defaults are 740 MHz / 430 mV, a 75°C hottest-chip limit, Automatic performance
off and a 30% normal fan floor. The learned spike reserve is capped at 3°C.
The sections below are dated observations of earlier builds and settings; they
do not override the current [operating policy](automatic-performance.md).
Invalid nonces no longer trigger a safety stop. The zero-error rule described
in the historical clock qualification was the policy used for that test.

## Latest clock qualification

The latest [fixed-voltage clock qualification](clock-characterization.md) on
the 2.2.1 test unit retained 755 MHz at 430 mV after twenty minutes without
hardware errors. 760 MHz produced a new hardware error and was automatically
rejected. The per-unit automatic ceiling is now 755 MHz; other units retain
the initial 740 MHz ceiling. Normal cooling now has a 30% minimum and faster
temperature-dependent fan reductions. The recorded run preserved the 80°C
limit, original passwords and raw thermal safeguards, and ended with default
740 MHz / 430 mV mining and fresh DATUM acceptance.

## Remote SSH activation

**2.2.1 is the newest official original KD BOX image found with the ordinary
dashboard maintenance route for starting SSH.** Offline inspection traced
`PUT /mcb/protect` to the same stock SSH start command in 2.1.3, 2.2.0, and
2.2.1. It returned without starting SSH in 2.2.3 and 2.2.4. Goldshell's
original KD BOX image directory lists no 2.2.2. The ordinary dashboard request
opened port 22 and allowed an authenticated SSH session on the live test unit
running both 2.1.3 and 2.2.1.
[Official 2.2.1 image](https://github.com/goldshellminer/firmware/blob/master/KDA-Miner/Goldshell-KDBox/firmware-Goldshell-KDBox-2.2.1.cpb) · [Original KD BOX image directory](https://github.com/goldshellminer/firmware/tree/master/KDA-Miner/Goldshell-KDBox)

## SPECTRE WORKS live installation check

On 2026-11-10, the installer opened SSH from a closed port on one original
MCB_V5_4 KD BOX, matched the stock 2.2.1 management hash, saved a verified
application snapshot, and started KD BOX Control. The controller, SSH, and
both original passwords worked after one reboot. Both fans exceeded 4,700 RPM
at full output after settling. The unit had no configured pools or mining
telemetry. The controller stopped mining after the telemetry timeout and kept
the dashboard available. This installation check ended before a pool was
configured.

### Chip temperature on the stopped test unit

The stock mining log recorded a 47.0°C chip average, a 52.0°C chip maximum,
and a 36.4°C hashboard temperature five seconds before a later safety stop.
The stock management service builds `/dbg/icinfo` from the mining engine's
`devdetails` response, copying each `cN-temp` field into a per-chip `temp`.
The controller received the report and rejected at least one missing or invalid
`temp` entry. Its stop message shows 0.0°C when that check fails. The saved stop
record contains the aggregate readings and error, so it cannot identify the
affected chip or its reported value.

With the mining engine held stopped, `/dbg/icinfo` returned HTTP 500 and the
stock Mining log report was empty. In **Diagnostics**, the saved **Safety stop**
report is available while controls are locked. Unlock controls to view the
**Mining log**, which reads recent entries from the on-device stock log when
the stock report is empty or unavailable. Future temperature stops record the
affected board and chip row when the firmware returns a missing or invalid
temperature. The existing stop record is preserved.

### Kadena attempt before XBT verification

The stock log identifies the engine as `kdaminer` on this run. It detected
one hashboard with 16 chips and the saved telemetry showed about 1.21 TH/s
before the temperature stop. The configured DATUM gateway accepted a Stratum
connection, but the miner recorded five `unknown-work` rejections and zero
accepted shares. The gateway serves Bitcoin BLAKE2b work, which requires the
dashboard's **Bitcoin (XBT)** selection and the stock `blake2b(SC)` engine.

Bitcoin (XBT) was saved for the next resume while the miner remained stopped.
The saved Kadena log contains aggregate temperatures and does not identify the
chip entry that triggered the earlier stop.

### Supervised XBT resume on stock 2.2.1

The same unit completed a supervised resume using the confirmed 740 MHz,
430 mV, and fixed 100% fan profile. The stock supervisor calibrates the fans
before starting the mining engine. Once `intminer` started, all 16 chip
temperatures initially read 0°C and became valid about 12 seconds later.

At the end of 150 seconds of observation from resume, the miner and its DATUM
client each recorded seven accepted shares and zero rejections. Across 39
complete chip reports, the highest observed chip temperature was 62°C. The
final chip report ranged from 46°C to 62°C. The controller's thermal thresholds,
approved tuning, and existing passwords were preserved.

### Cold XBT resume after the startup fix

The live test began with the stock supervisor stopped and a stale engine PID
file present. One **Resume mining** request cleared the stale PID and started a
fresh stock supervisor. The stock fan sweep ran while the engine was off. XBT
started about 86 seconds after Resume, and all 16 chip temperatures became valid
about 13 seconds later. The controller restored full fan output on its own as
the engine started. The miner and its DATUM client each recorded eight accepted
shares and zero rejections over the 152-second test. The highest chip reading
was 66°C. The approved tuning and original passwords matched their pre-test
checksums.

### Automatic cooling on the test unit

A 276-second live run verified automatic cooling with the existing 740 MHz
and 430 mV settings. Both fan outputs reduced from 100% to a settled 51%, with
3,000 and 2,940 RPM at the end. The controller compensates for the stock PWM
driver rounding below its requested duty, maintaining the configured 50%
minimum. The final hashboard temperature was 55°C. The highest observed chip
temperature was 76°C across 121 complete chip reports. The miner and DATUM
recorded five accepted shares and zero rejections. Original password files and
confirmed clock and voltage settings matched their pre-test checksums.

Cooling was subsequently changed to use a hottest-chip limit of 70°C as well
as the hashboard. A chip reading of 75°C commands full fan output ahead of
the existing shutdown limits. In the live check, both fans reached 100% and
4,740 RPM, and the hottest-chip reading fell from 76°C to 68–70°C. The miner
and DATUM recorded 20 accepted shares with zero rejections.

### Smoothed hottest-chip control and 80% minimum

The initial smoothed cooling policy defaulted to a 70°C hottest-chip limit and an 80%
minimum during mining. Advanced tuning saves limits from 70–80°C independently
of clock and voltage trials. Normal adjustments use a 15-second temperature
filter, hysteresis and delayed, gradual reductions. Raw thermal checks retain
priority. Redundant normal PWM writes are skipped when the requested duty is
already reported; full-speed protection always writes immediately.

On the stock 2.2.1 test unit, the final build resumed XBT on the first request.
The stock fan calibration completed with the engine off, the XBT engine started
after approximately 86 seconds, and all 16 chip readings became valid after
approximately 98 seconds. A 338-second supervised run collected 161 complete
chip reports and confirmed 16 accepted shares on both the miner and its DATUM
client, with zero rejections. The highest observed chip reading was 76°C;
raw hot readings commanded full cooling while the filtered signal was still
below 70°C. Normal mining duty remained above the 80% floor.

During another 150 seconds of observation, raw hottest-chip readings ranged
from 66–72°C while the filtered signal stayed between 67.4–69.2°C. Fan output
eased gradually to 91%, with both fans reporting 4,500 RPM and the hottest chip
at 69°C at the end. A subsequent DATUM check confirmed 24 accepted shares and
zero rejections. Password files and confirmed clock/voltage settings matched
their pre-test checksums. Desktop and mobile browser checks on ports 80 and
8088 verified the overview and authenticated limit slider; 80°C persisted
through a reload, and the saved limit was restored to 70°C before mining.
These observations describe this test period on this unit.

### Dashboard ranges, autosave and brief chip spikes

The subsequent build separates Cooling from Clock & voltage, autosaves the
temperature slider after one second, replaces the status banner with an
indicator and tooltip, and uses blue accents. Desktop and mobile browser checks
passed on ports 80 and 8088, including 80°C persistence through reload, restoring
70°C, notification dismissal, and the live history controls. Simulated browser
checks covered automatic range growth, five-minute manual range expiry,
overlapping slider edits, and failed-save retries. The controller collected
additional timestamped history with the browser closed, using a private file
on the unit's `/tmp` tmpfs. The 24-hour retention bound was tested with synthetic
minute samples; the live test did not run for 24 hours.

The early hottest-chip boost now confirms raw limit +5°C readings for nine
seconds, or responds to sustained filtered heat. Near-limit cooling, raw thermal
shutdowns, fan checks and watchdogs retain priority. Regression tests verify
that brief 76–77°C chip excursions do not latch full output, sustained heat
does, and 85°C chip readings still command full output immediately on 2.2.1.

A 396-second live run resumed XBT on the first request and collected 201 complete
16-chip reports. The highest observed chip temperature was 73°C and the highest
chip average was 65°C. No full-speed boost was latched after sensor startup.
At the end, both fans reported 82%, at 4,260 and 4,200 RPM. The miner and its
DATUM client each confirmed 20 accepted shares and zero rejections. Original
password files and confirmed clock/voltage settings matched their pre-test
checksums. The saved limit remained 70°C and the mining duty floor remained
80%. This run did not encounter a raw chip reading at the former 75°C trigger;
the brief-spike case is covered by the regression test.

### SPECTRE automatic/default performance modes

The managed-performance build removes manual clock/voltage inputs and the
trial/confirm/revert API routes. Customers choose the 740 MHz / 430 mV SPECTRE
default or bounded automatic performance at full fan output. The initial
automatic bounds are 500–740 MHz and 425–430 mV. The pre-install 850 MHz /
430 mV setting is an observed stock configuration, not a qualified XBT default.
See [the control policy](automatic-performance.md) for qualification and recovery.

On this stock 2.2.1 unit, a 785-second supervised run collected 449 complete
16-chip reports. Automatic mode first established its 740 MHz / 430 mV baseline,
then requested 740 MHz / 425 mV after five minutes of stable observations and
accepted work. Mining restarted cleanly, temperatures returned, and reported
clock/voltage matched the new point. After the automatic verification window,
the retained point had zero hardware errors across 4,853 nonces, with 18 accepted
shares and zero rejections confirmed by its DATUM client. Both fans reported
100% and 4,740 RPM at that observation.

Turning automatic mode off restored 740 MHz / 430 mV without a customer
confirmation step. Mining restarted cleanly again, and four new accepted shares
with zero rejections were confirmed on DATUM during the final default-mode check.
Across the test, the highest chip reading was 73°C and the highest chip average
was 63°C. Original passwords and protected executables matched their checksums;
the saved temperature limit remained 70°C. Automatic mode was left off with
the default profile active. Desktop/mobile checks verified neutral surfaces,
the softer blue accents, the new 3h range, and both mode selections on ports
80 and 8088.

This live run exercised the two 740 MHz voltage points and restoration of the
default. Thermal frequency decisions, bounds, rejected-setting recovery and
missing-chip rejection were covered by controller tests. It is not live
characterization of every point in the automatic range or certification of
other boards, PSUs, assemblies or ambient conditions.

### Normal cooling floor restored to 50%

The normal cooling minimum was subsequently returned to 50%. Automatic
performance retains full fan output, and thermal limits, filtering and
hysteresis are unchanged. The live unit's saved 80°C limit, default performance
mode, credentials and approved tuning were preserved byte-for-byte during the
update. Desktop/mobile checks on both dashboard ports showed the 50% minimum.
A 363-second supervised run collected 179 complete chip reports, confirmed
19 accepted shares on the miner and DATUM with zero rejections, and observed
fan output pass below the former floor to 79% at 4,140 RPM on both fans. The
highest chip reading during that run was 71°C. This verifies operation below
the old floor; output continues to follow temperature rather than being forced
to 50%.

### Warming prediction and cooling settling

The [cooling investigation](cooling-control.md) reproduced fan reductions to
33–34% during warming, followed by an 85°C protective boost and an observed
86°C hottest-chip reading. Normal control now anticipates warming, reserves
bounded headroom for hottest-chip excursions, and lets airflow changes settle.
Raw thermal limits and the strict hardware-error guard remain active. The
record includes a separate native invalid-nonce event at the default clock;
do not interpret a short clean run as indefinite hardware qualification.

## Upstream evidence

> **Evidence provenance:** The results below are preserved upstream reports from
> `the upstream project` at `5589e0604596722896266f7f0d565c1cc4484118`.
> They describe upstream tests on one original KD BOX.


The 0.1.0 prerelease supports original Goldshell KDBox firmware **2.2.1** only.
The 0.1.1 prerelease supports **2.0.6**, plus **2.0.5, 2.0.7q, 2.0.8, 2.0.9, 2.1.0, 2.1.1, 2.1.3, 2.2.0 and 2.2.3** after installation, accepted-share,
pool-display and reboot tests on an original KDBox / MCB_V5_4.

Other Goldshell models and control boards are not covered by these results.
A bundled mining engine or a known SSH password does not establish compatibility.

## Firmware inventory

All images below came from the [official Goldshell firmware repository](https://github.com/goldshellminer/firmware/tree/master/KDA-Miner/Goldshell-KDBox).
Their firmware payload signatures were verified against the vendor compatibility public key.

| Firmware | Installation and mining | SSH access |
| --- | --- | --- |
| 2.0.5 | Passed, including reboot | Legacy password matches firmware hash |
| 2.0.6 | Passed; reboot at 500 MHz and 740 MHz thermal run | Legacy password matches firmware hash |
| 2.0.7q | Passed, including reboot | Legacy password matches firmware hash |
| 2.0.8 | Passed, including reboot | Legacy password matches firmware hash |
| 2.0.9 | Passed, including reboot | 2.2.1 password matches firmware hash |
| 2.1.0 | Passed, including reboot | 2.2.1 password matches firmware hash |
| 2.1.1 | Passed, including reboot | 2.2.1 password matches firmware hash |
| 2.1.3 | Passed, including reboot | 2.2.1 password matches firmware hash |
| 2.2.0 | Passed, including reboot | 2.2.1 password matches firmware hash |
| 2.2.1 | Passed, including reboot | 2.2.1 password matches firmware hash |
| 2.2.3 | Passed, including fan trial and reboot recovery | Newer password verified over SSH |
| 2.2.4 | Unsupported: stock SSH access unavailable | Password matches firmware hash; SSH enable hook removed |

Firmware 2.2.3 uses a read-only root filesystem and a separate fan controller.
The installer preserves read-only boot behavior and gives KD BOX XBT sole control
of fan output. Rebooting during an unconfirmed tuning trial restored the previous
settings and mining resumed with accepted shares. Restoring the installation
backup also passed: the stock web page and startup configuration returned,
SSH remained recoverable, and the root filesystem booted read-only.

Firmware 2.2.4 removes the SSH-enable mechanism used by 2.2.3. It is rejected by
the installer. A stock-updater downgrade from 2.2.4 to the verified original KDBox
2.2.3 image succeeded on the test device, but downgrading is not automated by
the upstream application and carries the usual firmware flashing risks.

The [2026-11-10 investigation](firmware-2.2.4-investigation.md) independently
compares the official 2.2.3/2.2.4 firmware files and confirms the management
program's removed SSH-start branch. That offline analysis does not change the
hardware-validation scope of the upstream results above.

## Firmware 2.0.6 cooling

Installation defaults to 740 MHz, 430 mV and 100% fans. A staged clock test
followed by ten minutes at 740 MHz reached about 709 GH/s with accepted shares;
the highest observed chip temperature was 83°C. A separate installation and
reboot test passed at 500 MHz.

Shutdown remains set to 80°C average or 85°C chip temperature on this firmware.
These short tests do not guarantee thermal stability in a warmer environment.
Lower the clock if the device reaches its thermal limit; existing confirmed
settings are preserved when reinstalling.

## Scope of testing

An accepted share confirms that the tested miner submitted work accepted by its pool.
It does not guarantee long-term stability across every hardware revision or operating
environment. Use the installation backup and the conservative default tuning settings.

The installer does not upgrade or downgrade stock firmware. Do not flash a different
model's image or assume that a newer version preserves SSH or recovery access.

## Image checksums

SHA-256 of the downloaded CPB files:

```text
8d61a9c2ee41ae395480440e094a99e4d4043069da5d4579f522110344ca94d7  firmware-Goldshell-KDBox-2.0.5.cpb
3443b3235083a9a8fb8ab3e42bc75224b3a14b17ea814840d06ffdc89cd96d0a  firmware-Goldshell-KDBox-2.0.6.cpb
e461ce94fe799e6719f3733be1d47f354ce501fd486b7d6837cbfedb98461494  firmware-Goldshell-KDBox-2.0.7q.cpb
32740565bc88f5e130009568165886bb821f3844a85e855863f0ce20ea0e6526  firmware-Goldshell-KDBox-2.0.8.cpb
d86ea0621e96cc34c79500a898e8e47213a6154aa89e4e97fc02417e04008cbc  firmware-Goldshell-KDBox-2.0.9.cpb
4ed4e0345c9c6c754c04212e58c165ac7072dc70b5564aefbb34a011d2090004  firmware-Goldshell-KDBox-2.1.0.cpb
a4fbf61113bf627421e2d88ffd9c8b670bcbb26f3d5ea46006cb5339e66c6baf  firmware-Goldshell-KDBox-2.1.1.cpb
184b78aeb04646220b2a47776692af440550a4efdb6d9ea5ebe9eb0e4683df96  firmware-Goldshell-KDBox-2.1.3.cpb
a1dbfb6992c8508e7b971e343d44f6c3406b47e57c12906458ab6f550c5b7347  firmware-Goldshell-KDBox-2.2.0.cpb
5984af95c4434f2b20987548560263eb0b361674309e541f881551dcca55e87d  firmware-Goldshell-KDBox-2.2.1.cpb
dff68452c2102ff09978ed365868fde0044848a9dd9165fcadca44f7d9a7537c  firmware-Goldshell-KDBox-2.2.3.cpb
ed5a09cd7894d1bbff2b4c36a59dccfdfce606e2f084a2146fea4db8688b877f  firmware-Goldshell-KDBox-2.2.4.cpb
```
