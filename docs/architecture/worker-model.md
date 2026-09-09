# Worker Model

## What changed in v0.3.0?

The gateway still accepts telemetry and writes it to Kafka. A separate `worker`
process now joins the `telemetryforge-processors` consumer group and processes
those records.

This separation matters because HTTP ingestion and stream processing have
different scaling pressures. A burst of incoming requests can be absorbed by
Kafka without forcing the processing tier to grow at exactly the same rate.

## Data flow

```text
Client
  |
  v
Gateway
  |
  v
Kafka: telemetry.raw / telemetry.metrics
  |
  v
Consumer group: telemetryforge-processors
  |
  v
Bounded in-memory queue
  |
  +---- Worker 1
  +---- Worker 2
  +---- Worker 3
  `---- Worker N
```

## Why a bounded queue?

The queue is deliberately finite.

If the database or a future enrichment service becomes slow, an unlimited Go
channel would simply keep accepting records until the process ran out of
memory. TelemetryForge instead stops pulling work into memory when the queue is
full. Kafka remains the durable backlog.

This is backpressure rather than failure.

## When is an offset committed?

Kafka auto-commit is disabled. A worker commits a record after processing
succeeds. If processing fails, the record is left uncommitted so it can be
retried after a restart or rebalance.

This gives v0.3.0 **at-least-once processing**, not exactly-once processing.
Processors therefore need to become idempotent as persistence is added.

## What does the processor do today?

The v0.3.0 `Normalizer` intentionally does very little: it trims accidental
whitespace from source and event type. The important part of this release is
the processing architecture, not a large collection of arbitrary transforms.

Future processors will add schema validation, enrichment, aggregation,
persistence, retry classification, and the Incident Flight Recorder.
