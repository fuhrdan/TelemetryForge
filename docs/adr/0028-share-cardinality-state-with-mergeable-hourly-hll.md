# ADR 0028: Share Cardinality State with Mergeable Hourly HLL

**Status:** Accepted  
**Date:** 2026-09-09

## Context

Replica-local cardinality state under-counts a stream when Kafka partitions are
processed by multiple worker replicas.

Storing every unique raw value centrally would provide exact counts but would
create unbounded state and copy potentially sensitive identifiers into another
dataset.

## Decision

Production workers use a PostgreSQL/TimescaleDB shared hourly state containing:

- 64 HLL registers;
- at most 16 hashed exact values;
- first/last timestamps; and
- sample count.

Each observation atomically raises one HLL register using `max(existing,rank)`.

The key includes tenant, active/shadow mode, source, type, dimension, and hourly
window.

Replay/cost simulation keeps an isolated local tracker using the same hourly
window semantics.

Shared state has a seven-day retention policy.

## Alternatives considered

### Redis HyperLogLog

A viable distributed estimator, but it would introduce another mandatory
production service when PostgreSQL/TimescaleDB is already a required worker
dependency.

### Exact database set of hashes

Accurate but storage grows with cardinality, which makes the control-plane state
itself vulnerable to the problem being measured.

### Replica-local HLL plus summation

HLL estimates cannot be safely summed because replicas can observe overlapping
values. Registers must be merged.

## Consequences

Workers now make materially consistent cluster-wide decisions against the same
shared register state.

The database receives more writes. Later scale work may batch register deltas,
but it must preserve mergeability and policy semantics.

A database outage does not silently switch production policy to local state.
