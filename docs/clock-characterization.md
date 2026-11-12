# Fixed-voltage clock qualification — 2026-11-11

The original KD BOX / MCB_V5_4 at `192.168.86.181`, running pinned stock
2.2.1 with XBT / `intminer`, qualified **755 MHz at 430 mV** during this test.
**760 MHz produced a new hardware error** and was automatically rejected.
755 MHz is the highest zero-error point observed in this bounded search,
resolved to 5 MHz. It is a per-unit operating ceiling for the recorded test
period, not an absolute silicon maximum or a guarantee for other hardware.

Subsequent [cooling investigation](cooling-control.md) recorded one native
invalid nonce at the 740 MHz / 430 mV default, with a 72°C hottest chip. The
unchanged quality guard stopped mining. That event and subsequent supervised
testing are recorded separately; this short qualification is not evidence of
indefinite error-free operation at either the default or the higher ceiling.

## Conditions and method

- Core voltage stayed at the reported **430 mV** throughout. Live changes used
  only the clock setter; full-profile recovery retained 430 mV. The customer
  cannot select numeric clock or voltage values.
- Automatic performance commanded both fans to **100%**. Readback was 99–100%
  because of the stock PWM driver's rounding. The user's **80°C** hottest-chip
  limit was preserved.
- Each increase needed five minutes of fresh complete 16-chip temperature
  reports, at least 1,000 new nonces, a new accepted share and zero new hardware
  errors. A 10 MHz search found a rejection at 760 MHz; a 5 MHz refinement then
  tested 755 MHz.
- 755 MHz passed five-minute qualification and another **900-second
  sustained hold**, about **20.0 minutes total**, with
  **zero hardware errors**, **20,090 additional nonces**, and
  **55 additional miner-accepted shares**. DATUM independently
  confirmed acceptance for the miner's client session.
- Raw temperature shutdowns, fan RPM checks, watchdogs, missing-sensor handling
  and recovery journals remained enabled. The 760 MHz rejection was the
  hardware-error check, not a thermal shutdown.

The 755 MHz observation ran from **2026-11-11T21:16:25.206+00:00** through
**2026-11-11T21:36:22.268+00:00** (host UTC). Its highest observed chip reading was
**77°C**. Ambient temperature and input power were not instrumented;
this run does not qualify PSU limits or warmer operating conditions.

## Observations

| Clock (MHz) | Observation | New nonces | New accepted shares | Highest chip | Outcome |
| --- | --- | ---: | ---: | ---: | --- |
| 740 | 308 s sampled | 5,066 | 15 | 75°C | Zero hardware errors |
| 750 | 303 s sampled | 5,071 | 15 | 76°C | Zero hardware errors |
| 760 | About 83 s until rejection | 1,260 | 2 | 74°C | New hardware error; automatically restored 750 MHz |
| 750 (recovery) | 303 s sampled | 5,117 | 9 | 75°C | Zero hardware errors |
| 755 | 1197 s sampled | 20,090 | 55 | 77°C | Zero hardware errors |

Counts are deltas within each uninterrupted engine segment. The recovery
restart reset the native mining counters. Polling was approximately every
1.5 seconds; the controller checks its own fresh telemetry and rejected the
first new error before the next retained status sample. The failed point's
exact error count is therefore not inferred from the sampled table.

## Saved result and final state

The unit's private configuration now caps automatic performance at **755 MHz**,
with voltage fixed at **430 mV**. The rejected 760 MHz point will not be retried.
New installations still start with a 740 MHz ceiling; this result is not
applied globally. See [the operating policy](automatic-performance.md).

After qualification, automatic performance was returned to its original **off**
state, restoring **740 MHz / 430 mV**. Fresh chip temperatures and new accepted
work on both the miner and DATUM were verified after restoration. The saved
80°C limit and original passwords matched their pre-test values; protected
password files and executables matched their checksums. No firmware flashing
or reboot was used.

The deployed build also places Board temperature next to the fan cards,
sets the normal-cooling minimum to **30%**, and reduces fan output by two
percentage points per five seconds with ample headroom, or one point near
the limit. The 20-second cool dwell and thermal overrides remain. Live normal
cooling reduced from approximately 99% to 47% before the full-fan clock test;
the controller retains authority to stay above 30% when temperatures require it.

Controller SHA-256:

```text
aead56c48bd8e3c0cfbaa7a1572abae850c75dac1a101b88cff7a616ec49d66f
```

The local build, Go tests/race detector/vet, 57 Python tests and JavaScript syntax
checks passed. Browser checks verified the deployed UI on ports 80 and 8088,
including desktop/mobile card order, chart controls, tooltips and theme styling.
Private raw samples, chip CSVs, DATUM client observations and the application
rollback snapshot were retained on the test host outside the repository.
