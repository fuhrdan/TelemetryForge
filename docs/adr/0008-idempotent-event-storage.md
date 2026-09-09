# ADR 0008: Use a Dedicated Event-ID Deduplication Table

**Status:** Accepted  
**Date:** 2026-09-09

## Context

v0.3.0 established at-least-once Kafka processing. A worker may process the
same record again after a crash or rebalance.

A TimescaleDB hypertable's unique constraint must include its partition key, so
a simple global unique constraint on only `event_id` is not available there.

## Decision

Reserve every canonical event ID in the normal PostgreSQL `event_dedup` table
and insert the time-series row in the same transaction.

If the event ID already exists, persistence succeeds as a no-op.

## Consequences

Retrying an already-persisted Kafka record is safe. The extra table adds one
small relational write per new event. Later retention rules must consider how
long deduplication IDs need to remain useful.
