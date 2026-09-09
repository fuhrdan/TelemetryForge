# ADR 0009: Classified Retries and a Kafka Dead-Letter Queue

**Status:** Accepted  
**Date:** 2026-09-09

## Context

Retrying every failure wastes capacity and can wedge partitions on poison
records. Never retrying makes short dependency outages unnecessarily noisy and
fragile.

## Decision

Classify processing errors as transient or permanent.

Transient errors receive bounded exponential backoff with jitter. Permanent
errors skip retries. Exhausted/permanent failures are published to
`telemetry.dlq`.

The original Kafka offset is committed only after the DLQ publish succeeds.

## Consequences

Poison events no longer block a partition forever. Short dependency failures
can recover locally. The DLQ becomes an explicit operational backlog requiring
monitoring and replay procedures.
