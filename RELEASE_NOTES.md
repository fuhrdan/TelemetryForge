# TelemetryForge v1.4.0 Development Release Notes

## Adaptive Sampling & Telemetry Shaping

v1.4.0 reduces healthy high-volume telemetry without weakening the evidence-first
incident model.

## Deterministic sampling

Sampling uses config identity plus canonical event ID. A compact durable
decision ledger records the first queue-pressure decision so whole-chain retries
reuse it and minute statistics are incremented exactly once.

## Protected telemetry

The checked-in active configuration protects errors, severe events, audit/
deployment/security events, incident-tagged telemetry, and >=1000ms latency/
duration signals. Protected events are never sampled out by queue pressure and
preserve tags/payload unless a rule explicitly opts into `shape_protected`.

## Pressure adaptation

The active configuration uses bounded worker queue utilization to lower healthy
traffic sampling rates at 70% and 90% queue pressure. Per-rule floors prevent
unbounded reduction. Kafka remains the durable backpressure boundary.

## Telemetry shaping

Rules can drop tags, rename tags, and drop oversized JSON payloads while adding
a safe `telemetryforge.payload_oversize=true` marker. Arbitrary byte truncation
is not used because it could create invalid JSON.

## Full-fidelity / fail-open safety

Flight Recorder, Schema Intelligence, and the Cardinality Firewall run before shaping, so sampling cannot hide cardinality explosions from shared estimates. A sampled-out event
is acknowledged as a successful policy decision rather than sent to a DLQ.

Active shaping is applied only after its compact per-event decision is durable.
The ledger contains no telemetry payload. If that write fails, TelemetryForge
keeps the original unshaped event.

## Shadow shaping

`shaping/shadow.json` is evaluated against the same pre-shaped event and queue
pressure but never mutates production output. Only candidate differences are
stored.

## Visibility preview

`telemetryctl shaping preview` evaluates active/candidate shaping against a
frozen incident and reports explicit event, byte, protected-event, and
source/type retention. No opaque visibility score is generated.

## Dashboard / API / metrics

Added:

```text
GET /api/v1/shaping/stats
GET /api/v1/shaping/shadow-diffs
telemetryforge_shaping_decisions_total
telemetryforge_shaping_queue_pressure_ratio
```

The Next.js dashboard shows last-hour retention, protected/transformed counts,
and active-vs-shadow sampling changes.

## Demo

```bash
make demo-shaping
```

The demo mixes ordinary request-duration telemetry with protected errors/high
latency and oversized payloads.

## Known limitations

- Queue pressure is process-local, not cluster-global.
- v1.4 is event-aware sampling, not full trace-tail sampling/assembly.
- Shaping evidence is not auto-pruned; `telemetryctl shaping prune` provides an
  explicit guarded lifecycle operation with a 35-day minimum horizon.
- Payload shaping supports safe whole-payload dropping rather than arbitrary
  JSON-path transforms.
- Full dependency-backed Go 1.27.1 and TimescaleDB validation remains CI-
  authoritative when unavailable in a restricted sandbox.
