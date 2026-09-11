# Evidence-Backed Policy Recommendations

TelemetryForge v2.0.0 can produce a narrow advisory recommendation when recorded frozen-incident analysis shows a material candidate-policy effect.

The initial recommendation type is intentionally constrained: evaluate an already-recorded **shadow policy** for controlled promotion when the latest cost simulation projects at least a 5% canonical-byte or sampled-series reduction.

## Required evidence

A recommendation cites the exact cost-simulation ID. When a completed shadow replay is present, it cites that replay ID too. Quarantined replay events add an explicit safety warning.

The recommendation reports the measured directional effects from the frozen incident sample, for example:

```text
canonical_bytes_reduction_percent
sample_series_reduction_percent
```

These are not vendor billing promises or universal capacity guarantees.

## Human-control boundary

Every production-affecting recommendation has:

```json
{
  "requires_human_approval": true,
  "lifecycle_action": "shadow -> evidence review -> approval -> activation"
}
```

The intelligence package has no method that activates lifecycle state or rewrites production policy. Promotion remains in the v1.6 immutable configuration lifecycle and requires its existing control-scope authorization and human approval/evidence gates.

If a completed replay is absent, the recommendation explicitly tells the operator to run Incident Replay before promotion.

## Non-goals

v2.0 does not:

- autonomously rewrite a policy;
- autonomously activate a policy;
- infer dollar savings without explicit pricing;
- conceal contradictory evidence;
- recommend a change from insufficient evidence merely to produce an answer.
