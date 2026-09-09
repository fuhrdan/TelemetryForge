# ADR 0004: Partition telemetry by source

- **Status:** Accepted
- **Date:** 2026-09-09

## Context

Kafka only guarantees ordering within a partition. TelemetryForge needs useful ordering without concentrating all telemetry into one partition.

## Decision

Use `event.source` as the Kafka record key.

The producer uses Kafka-compatible key hashing, so records from the same source are consistently routed to the same partition while unrelated sources can be distributed across the topic.

## Why not event ID?

An event ID is effectively random and would spread records well, but it would destroy source-local ordering.

## Why not correlation ID?

Correlation IDs are optional and often short-lived. They are valuable as headers for downstream correlation but are not stable enough to define the primary partitioning strategy.

## Consequences

- Per-source ordering is preserved within a topic.
- A single extremely hot source can create a hot partition.
- Later releases may support configurable partition policies for high-volume tenants.
