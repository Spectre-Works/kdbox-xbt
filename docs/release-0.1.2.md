# 0.1.2 prerelease

Historical upstream/fork notes. Use the [1.0 README](../README.md) and
[current release notes](release-1.0.md) for installation and operating policy.

- Retry transient SSH connection failures with a fresh connection and bounded backoff.
- Give slow SSH servers more time to send their greeting.
- Stop immediately on rejected passwords, declined identity confirmation or a changed host key.
- Show connection attempts and actionable recovery instructions.
- Retry only explicitly read-only device API requests.
- Verify downloaded backup files and staged uploads before replacement.
- Check dashboard startup and fresh telemetry before reporting installation success.
- Keep the window open during installation/recovery and fix queued progress messages.
- Hide entered passwords in displayed error messages.
- Retry transient, read-only stock-controller requests once without replaying writes.
- Preserve the original safety-stop reason, time and available telemetry across restarts.
- Reapply the existing algorithm when resuming a stopped mining engine.
- Refuse installation before file changes when a safety stop is still active.

Firmware support is unchanged from 0.1.1. No automatic firmware upgrade or downgrade
is performed. Fault-injection tests cannot guarantee recovery from hardware failure.
