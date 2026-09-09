# TelemetryForge Architecture — v0.6.0

TelemetryForge now has three application surfaces:

1. Go ingestion/query gateway.
2. Go Kafka processing workers.
3. Next.js browser dashboard.

Shared infrastructure remains Kafka plus PostgreSQL/TimescaleDB.

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

The gateway validates ingestion, waits for Kafka acknowledgement, exposes
bounded query APIs, dashboard summaries, incident reads, and the SSE stream.

## Worker

The worker uses consumer groups, bounded queues, classified retries, the Flight
Recorder, normalization, idempotent persistence, and automatic incident
detection.

## Dashboard

The Next.js dashboard is a separate service. It does not connect directly to
Kafka or the database. All browser data flows through the Go API boundary.

The browser uses a same-origin proxy path so v0.6.0 does not need permissive
CORS configuration.

## Horizontal-scaling principle

The dashboard live stream reads shared durable storage rather than process-local
memory. This preserves correct behavior when gateway/worker replicas are added
in v0.7.0.

## Next milestone

v0.7.0 adds Kubernetes scaling and the Cardinality Firewall. The dashboard and
SSE contract established here should continue working as replica counts grow.
