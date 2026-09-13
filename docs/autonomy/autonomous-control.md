# Autonomous Control

TelemetryForge v2.7.0 adds a bounded control loop around the existing adaptive-shaping pipeline.

## Modes

- `off`: no forecasting or runtime adjustment.
- `shadow`: forecast pressure and record the action that would be taken, but keep the active multiplier at `1.0`.
- `auto`: explicit opt-in. A forecast crossing the configured threshold may apply a temporary non-protected sampling multiplier.

`auto` is not a configuration editor. It cannot promote lifecycle artifacts, rewrite routing, change durability, or disable protected telemetry.

## Predictor

The built-in predictor uses a transparent least-squares pressure trend over a bounded observation window and projects that trend over a configured horizon. The output includes current pressure, predicted pressure, slope, sample count, and a coarse confidence label. This is intentionally explainable rather than an opaque root-cause model.

## Guardrails and rollback

An automatic action is bounded by a minimum multiplier, TTL, recovery threshold, error-rate regression threshold, latency-regression threshold, and cooldown. Before application the action is fsynced to the local JSONL audit trail. Audit failure suppresses mutation.

Rollback restores the multiplier to `1.0` when any guardrail fires. Shaping decisions persist the multiplier that was used, so a worker retry reconstructs the same decision even if the live controller has since changed.

## Status

The worker admin listener exposes:

```text
GET /autonomy/status
```

The response contains the current forecast, multiplier, active action, recent terminal actions, and cooldown state.

## Configuration

See [Configuration Reference](../reference/configuration.md). `TELEMETRYFORGE_AUTONOMY_MODE` is disabled by default in the binary. Start with `shadow`; enable `auto` only after reviewing proof output and workload-specific thresholds.
