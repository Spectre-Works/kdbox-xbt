# KD BOX XBT repository

Read README.md and the relevant architecture/installation documentation before
changing the installer or controller. Preserve owner passwords, pool/network
settings and genuine safety stops unless the owner explicitly requests a change.

## Documentation scope

Keep repository documentation about the product: supported hardware/firmware,
operating policy, architecture, reusable validation procedures and release notes.
Do not commit unit-specific testing or troubleshooting records, private LAN IPs,
serial/MAC identities, benchmark histories, deployment logs, credentials or recovery
snapshots. Keep that evidence in private, off-repository storage. Report finite
live verification results in the task response; do not turn them into repository
claims of general hardware reliability.

## Validation and releases

Use the local commands in README.md (Go toolchain 1.27.3 or newer). Run relevant
checks before committing; build release artifacts from clean committed source.
Do not include vendor firmware, mining binaries, keys or private QA artifacts in
public packages.
