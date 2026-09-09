# TelemetryForge Documentation

This directory is the human-readable companion to the source code. The goal is
for a reviewer to understand why the system behaves as it does without having
to reconstruct every decision from implementation details.

## Start here

- [Architecture overview](architecture/overview.md)
- [Event flow](architecture/event-flow.md)
- [Configuration reference](reference/configuration.md)
- [Local development](development/local-development.md)
- [Testing strategy](development/testing.md)
- [Troubleshooting](operations/troubleshooting.md)
- [v0.7.0 development plan](roadmap/v0.7.0.md)

## Architecture

- [Overview](architecture/overview.md)
- [Worker model](architecture/worker-model.md)
- [Backpressure](architecture/backpressure.md)
- [Failure handling](architecture/failure-handling.md)
- [Event flow](architecture/event-flow.md)

## Kafka

- [Topic strategy](kafka/topic-strategy.md)
- [Partitioning](kafka/partitioning.md)
- [Delivery semantics](kafka/delivery-semantics.md)

## Storage

- [Schema](storage/schema.md)
- [Retention](storage/retention.md)
- [Querying](storage/querying.md)

## Reliability and incidents

- [Retry policy](reliability/retries.md)
- [Dead-letter queue](reliability/dlq.md)
- [Deduplication lifecycle](reliability/dedup-lifecycle.md)
- [Incident Flight Recorder](reliability/flight-recorder.md)
- [Automatic incident capture](incidents/automatic-capture.md)

## Dashboard

- [Dashboard overview](dashboard/overview.md)
- [Server-Sent Events](dashboard/sse.md)

## Operations

- [`telemetryctl`](operations/telemetryctl.md)
- [Troubleshooting](operations/troubleshooting.md)

## Development

- [Local development](development/local-development.md)
- [Testing](development/testing.md)
- [Release process](development/release-process.md)
- [GitHub repository conventions](development/github-repository.md)

## Reference

- [HTTP API](api/http-api.md)
- [Event format](api/event-format.md)
- [Configuration](reference/configuration.md)
- [Technology versions](reference/versions.md)

## Architecture Decision Records

ADRs are numbered sequentially in [`adr/`](adr/). Add an ADR when a change
materially affects deployment, reliability, data semantics, security boundaries,
or long-term maintainability.
