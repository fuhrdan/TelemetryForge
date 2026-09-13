# TelemetryForge v2.7.0 Release Notes

## Autonomous Control

TelemetryForge v2.7.0 adds a shadow-first, reversible control loop to the worker while preserving every durability, routing, lineage, formal-verification, and fast-path boundary from v2.1-v2.6.

### Pressure prediction

The built-in controller observes bounded worker queue depth and completed-job SLO signals and uses a transparent least-squares trend projection over a bounded window. Forecasts expose current pressure, predicted pressure, slope, sample count, and confidence.

### Safe action boundary

The binary defaults to `off`. In `shadow`, the controller records the action it would take without mutation. In explicit `auto`, it may apply only a TTL-bounded multiplier to non-protected shaping rates. Existing rule minimums and all protection rules still apply.

Actions are fsynced to a local JSONL audit file before the multiplier changes. An audit failure suppresses mutation. Automatic rollback restores `1.0` on pressure recovery, TTL expiry, error-rate regression, or mean-processing-latency regression, followed by cooldown.

### Retry stability

Each shaping decision records the autonomy multiplier used. If downstream processing retries the event after the controller has changed, TelemetryForge reconstructs the original keep/drop decision with the original multiplier.

### WebAssembly predictor boundary

`telemetryforge-plugincheck` validates digest-pinned WebAssembly binary-format-v1 modules and requires the declared exported entrypoint. The v2.7 host contract is capability-free and limited to bounded JSON input/output plus a deadline. No third-party Wasm engine is bundled; execution sandboxes remain an explicit backend responsibility.

### Operational evidence

`proof/autonomous-control.sh` tests shadow non-mutation, bounded auto action, SLO rollback, protected telemetry, audit transitions, and the Wasm validator and emits a `.tfproof.json` artifact.

### Explicit non-claims

v2.7 does not claim general artificial intelligence, root-cause certainty, globally coordinated multi-worker control, universal cost savings, autonomous lifecycle promotion, or safe execution of arbitrary third-party Wasm engines.
