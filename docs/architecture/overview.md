# TelemetryForge Architecture Overview

TelemetryForge currently has three application surfaces:

1. a Go ingestion/query gateway;
2. Go Kafka processing workers; and
3. a Next.js browser dashboard.

Shared infrastructure is Apache Kafka plus PostgreSQL/TimescaleDB.

```text
Applications
    |
    v
Go Gateway -------------------------------+
    |                                     |
    v                                     | query / SSE
Kafka                                     |
    |                                     |
    v                                     v
Worker Group                         TimescaleDB
    |                                     ^
    v                                     |
Flight Recorder                           |
    |                                     |
    v                                     |
Normalize -> Persist ---------------------+
    |
    v
Automatic Incident Detector
    |
 threshold breach
    v
Frozen Incident
    |
    +------------------------------------> Dashboard

terminal processing failure -> telemetry.dlq
```

## Gateway

The gateway owns the public HTTP boundary. It:

- validates the canonical telemetry envelope;
- waits for Kafka acknowledgement before returning HTTP `202`;
- exposes bounded event/metric queries;
- returns dashboard summaries and incident data; and
- serves the SSE live-data contract.

## Kafka boundary

Kafka separates bursty ingress from downstream processing.

Records are keyed by source. One source therefore retains Kafka partition order,
while independent sources can spread across partitions.

Consumer workers use manual acknowledgement. Concurrent completions are
coordinated per partition so a later offset cannot commit past earlier
unfinished work.

A bounded `PollRecords` batch also holds its current partition ownership until
all submitted jobs have completed normal or dead-letter handling. The consumer
then allows a group rebalance.

## Worker

The current processing chain is:

```text
Flight Recorder
    -> normalization
    -> idempotent TimescaleDB persistence
    -> automatic incident detection
```

The worker pool is bounded. Backpressure therefore grows as Kafka lag instead
of unbounded Go heap usage.

Transient failures retry with bounded exponential backoff and jitter. Permanent
or exhausted failures are written to `telemetry.dlq`.

## Storage

PostgreSQL/TimescaleDB provides:

- time-series telemetry storage;
- global event-ID deduplication;
- the rolling full-fidelity Flight Recorder;
- frozen incident metadata/events; and
- dashboard/query data.

The persistence boundary is idempotent so an at-least-once Kafka replay does
not create a duplicate primary telemetry row.

## Dashboard

The Next.js dashboard is a separate service. It does not connect directly to
Kafka or PostgreSQL.

All browser data flows through the Go API boundary. Live SSE data is sourced
from shared durable storage rather than process-local gateway memory, so the
contract remains valid when replicas are added.

## Scaling model

Two concurrency limits matter:

- **Kafka partitions** bound how many consumer-group members can actively own
  work at once.
- **Worker count** bounds how many records one consumer process can execute
  concurrently.

Adding Kubernetes replicas cannot create useful consumer concurrency beyond
available partitions. v0.7.0 documentation and manifests will make this
relationship explicit rather than treating HPA replica count as unlimited
throughput.

## Current development target

The `v0.7.0-dev` branch is preparing:

- Kubernetes deployments/services/configuration;
- health/readiness probes and resource requests;
- HPA and PodDisruptionBudget behavior;
- scaling guidance tied to Kafka partition count; and
- the Cardinality Firewall.

The stable released feature set remains `v0.6.0` until those acceptance criteria
pass and `v0.7.0` is tagged.

See the detailed plan in `docs/roadmap/v0.7.0.md`.
