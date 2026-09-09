# TelemetryForge v0.2.0 — Kafka Streaming & Durable Publishing

v0.2.0 changes TelemetryForge from an HTTP validation demo into an event-driven system with a durable streaming boundary.

## Highlights

- Accepted telemetry is now published to Apache Kafka.
- Generic events route to `telemetry.raw`; numeric metrics route to `telemetry.metrics`.
- The gateway returns `202 Accepted` only after Kafka acknowledges the record.
- Kafka failures surface as `503 Service Unavailable` rather than false acceptance.
- Events are keyed by source to preserve source-local order within Kafka partitions.
- `/ready` now reflects real Kafka reachability.
- Docker Compose boots a complete single-node KRaft Kafka environment and initializes topics automatically.
- CI includes a broker-backed Kafka integration test.

## Portfolio signal

This release demonstrates event-driven architecture, durable producer semantics, Kafka partitioning decisions, operational readiness checks, dependency abstraction, and infrastructure-backed testing.

## Next

v0.3.0 will add consumer groups, bounded Go worker pools, explicit backpressure behavior, graceful partition rebalancing, lag visibility, and the first downstream stream-processing pipeline.
