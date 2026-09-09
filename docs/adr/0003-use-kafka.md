# ADR 0003: Use Apache Kafka as the durable streaming backbone

- **Status:** Accepted
- **Date:** 2026-09-09

## Context

TelemetryForge needs to decouple high-concurrency ingestion from downstream processing. The streaming layer must support partitioned ordering, consumer groups, replay, durable retention, and horizontal scaling. These capabilities are foundational for later Incident Replay, shadow pipelines, and backpressure-aware workers.

## Decision

Use Apache Kafka as the primary streaming backbone beginning in v0.2.0.

The local development environment runs Kafka in KRaft mode. The gateway publishes synchronously and returns HTTP 202 only after Kafka acknowledges the record under the configured in-sync replica policy.

## Alternatives considered

### RabbitMQ

RabbitMQ is excellent for work queues and routing, but Kafka's log-oriented retention and replay model better matches TelemetryForge's future incident-replay requirements.

### Redis Streams

Redis Streams would reduce local infrastructure complexity, but Kafka provides a stronger long-term fit for partitioned scale, consumer groups, retention, and enterprise event-streaming interoperability.

## Consequences

- Local development gains another infrastructure dependency.
- Topic partitioning and retention become explicit architectural concerns.
- Consumer lag can later become a first-class backpressure signal.
- Durable log semantics create a natural foundation for replay and shadow processing.

## Client library

The Go gateway uses `franz-go` for Kafka protocol support. v0.2.0 pins a current stable release rather than wrapping a Java client or introducing CGO, keeping the gateway deployment as a single Go binary.
