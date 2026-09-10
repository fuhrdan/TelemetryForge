# Adaptive Sampling

TelemetryForge v1.4.0 adds deterministic, pressure-aware sampling **after** the
Flight Recorder, Schema Intelligence, and Cardinality Firewall have observed the
full incoming stream, but **before** normal persistence and Telemetry Router
fan-out.

The goal is to reduce healthy high-volume telemetry without turning sampling
into unexplained data loss. The Cardinality Firewall deliberately runs first so
sampling cannot hide an incoming series explosion from distributed estimates.

## Processing order

```text
Kafka event
   -> Flight Recorder (full fidelity)
   -> Normalizer
   -> Schema Intelligence
   -> Cardinality Firewall
   -> Adaptive Sampling / Shaping
   -> primary persistence
   -> Telemetry Router outbox
   -> Incident Detector
```

A sampled-out event stops successfully after the shaping stage. It is
acknowledged as handled; it does not enter retry or a DLQ.

## Deterministic selection

Sampling is based on a SHA-256-derived fraction of:

```text
config name + config version + canonical event ID
```

At a fixed pressure value, the same event/config produces the same deterministic
sampling result. In the live worker, the first durable pressure-aware decision is
reused across retries, so a later queue-pressure change cannot rewrite history.
This also keeps incident preview and policy review repeatable.

## Protected telemetry

The checked-in active policy protects:

- error event types;
- `error`, `critical`, and `fatal` severities;
- audit/deployment/security event types;
- incident-tagged telemetry; and
- latency/duration metrics at or above 1000 ms.

Protected telemetry cannot be sampled out even at critical worker queue
pressure.

Protection also preserves tags and payload by default. A matching rule must
explicitly set `shape_protected: true` before tag/payload transformations can
modify protected telemetry. Sampling can never exclude it.

## Queue-pressure adaptation

The pressure controller consumes the same bounded worker queue-depth signal used
for operational metrics.

The active default profile uses:

```text
< 70% queue utilization     base sample rate
>= 70%                      base rate × 0.60
>= 90%                      base rate × 0.25
```

Each rule can set `min_sample_rate`, so pressure cannot reduce it below the
reviewed floor.

This is local worker pressure, not a cluster-wide global load controller. Kafka
remains the durable backpressure boundary.

## Retry-stable, fail-open audit contract

Before TelemetryForge applies an active sample-out or transformation, it writes a
compact per-event shaping decision keyed by tenant, event, config name, and
config version. The row stores policy/effect metadata only, never the telemetry
payload.

The first decision wins. If a later processor fails and the worker retries the
whole chain, the sampler reuses the original observed queue pressure and decision
instead of changing keep/drop behavior. Minute aggregates increment only on the
first insert, so retries do not inflate sampling statistics.

If that decision/audit write fails, the worker keeps the **original unshaped
event** and continues. TelemetryForge therefore prefers extra telemetry over
unaudited loss.

Shadow-diff persistence is advisory; failure to record a candidate difference
does not change the active decision.

## Metrics

The worker exposes:

```text
telemetryforge_shaping_decisions_total{rule,outcome}
telemetryforge_shaping_queue_pressure_ratio
```

Allowed outcomes are bounded:

```text
kept
protected
sampled_out
```

Rule names originate only from validated configuration.


## Decision lifecycle

The retry-stability ledger and aggregate evidence are tenant-scoped. They are
not automatically deleted by the runtime. Operators can prune them explicitly:

```bash
telemetryctl shaping prune --older-than 840h --tenant default
```

The CLI refuses horizons shorter than 35 days so a shaping decision is not
normally removed before the corresponding dedup/replay investigation horizon.
