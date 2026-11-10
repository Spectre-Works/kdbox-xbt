# 0.1.2-sw.1 — SPECTRE WORKS preview

Based on upstream commit `5589e0604596722896266f7f0d565c1cc4484118`.

- KD BOX XBT Mod / KD BOX XBT Installer / KD BOX Control naming and neutral
  SPECTRE WORKS presentation; no upstream promotional logo or support link.
- Original MIT notice preserved; on-device licenses; build-specific Go notices;
  source-bearing packages with checksums and build/test dependency metadata.
- Frozen desktop runtime distribution replaced with editable installer source and
  a prebuilt ARM controller. Python/Tk/Paramiko are installed separately locally.
- Versioned complete managed-file snapshots, restricted local backup permissions,
  recovery allowlist validation, and checksum-guarded retirement of the known logo.
- Payload/ELF validation before GUI connection and installation writes; fixed
  asset allowlist rather than copying arbitrary files from the web directory.
- Diagnostic privacy warning and honest offline/recovery messaging. Runtime
  namespaces, compatibility gates, fan/thermal limits and watchdogs are retained.

No physical KD BOX validation is claimed by this release note. Complete
`installation-handoff.md` and record results before treating the fork as validated.
