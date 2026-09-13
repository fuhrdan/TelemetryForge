# ADR 0058: Autonomous control is shadow-first, bounded, and reversible

**Status:** Accepted for v2.7.0.

TelemetryForge may predict near-term worker pressure and recommend a temporary sampling adjustment, but autonomous control must not rewrite immutable lifecycle artifacts, bypass protected telemetry, or weaken the durable-ingest/replication/lineage boundaries.

The runtime controller therefore has three modes: `off`, `shadow`, and explicit opt-in `auto`. The only v2.7 automatic mutation is a time-bounded multiplier applied after the active shaping policy to **non-protected** telemetry. Existing shaping minimums still apply. Errors, configured severe telemetry, protected event types/tags, high-latency protected events, and other protection rules remain kept.

Every applied action must be durably appended to the local autonomy audit log before the multiplier changes. If audit persistence fails, the action remains shadow-only. Automatic actions roll back on TTL expiry, queue-pressure recovery, error-rate regression, or mean-processing-latency regression and then enter a cooldown.

WebAssembly predictor plugins receive a capability-free JSON boundary only. v2.7 validates module identity and an exported entrypoint and defines a sandbox backend interface, but does not give plugin code storage, network, lifecycle, configuration-mutation, or secret handles. A backend that executes Wasm must enforce memory/instruction isolation independently.
