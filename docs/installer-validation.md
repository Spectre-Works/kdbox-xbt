# Installer and controller validation

Use `python tools/local.py test` for the local build, Go tests/vet, Linux race
checks, Python tests and dashboard/installer JavaScript syntax checks. Use
`python tools/local.py all` from clean committed source to additionally build and
audit product and source archives. See the README for toolchain requirements.

## Installer flow

Verify discovery, manual-address fallback, supported base/board gates, SSH
fingerprint confirmation, current-password authentication, complete recovery
backups and existing-install/update/repair detection. Exercise refresh/session
recovery, clear validation errors, completion links and server shutdown.
Check desktop and narrow mobile layouts. Test host-specific launchers on their
actual supported platforms; simulated fixtures do not establish native OS support.

Check malformed requests, token/Host/Origin restrictions, bounded request bodies,
credential redaction, interrupted writes and rollback. Recovery must reject
incomplete or corrupt snapshots and retain genuine safety stops.

## Supervised hardware validation

Use an explicitly designated compatible miner and preserve its credentials and
saved settings. Record private before/after hashes and inspect existing faults
before any supervised resume. Installation success means the expected dashboard
is running; verify hashing and accepted pool work separately.

Check stock calibration, initial fan acceleration, both individual RPM readings,
fresh board/average temperatures and all 16 chip temperatures. Initial spin-up
uses a fixed ten-second deadline with the unchanged 3,000 RPM floor; a fan loses
its allowance once healthy. Sensor failures, deadline expiry and subsequent
stalls must stop mining. Temperature limits apply throughout startup.

Observe an uninterrupted mining session and increasing accepted shares,
preferably confirmed by the pool/gateway. Leave thermal, fan and watchdog
safeguards enabled. A finite observation is not hardware certification.
Keep all unit identities, measurements, logs and recovery snapshots outside
the repository, as required by AGENTS.md.

## Invalid-nonce policy

Regression tests cover isolated and large error counts in default mode, missing
quality counters, automatic qualification/error budgets, established-clock
backoff, counter resets, observation windows and the minimum clock floor.
Invalid nonces never request a safety stop; automatic performance may reduce
clock when error rates exceed its qualification policy. Thermal protection
remains independent. Do not deliberately induce hardware faults for testing.
