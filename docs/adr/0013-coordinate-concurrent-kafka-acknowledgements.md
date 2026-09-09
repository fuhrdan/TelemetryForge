# ADR 0013: Coordinate Concurrent Kafka Acknowledgements and Rebalances

**Status:** Accepted  
**Date:** 2026-09-09

## Context

TelemetryForge consumes Kafka records into a concurrent Go worker pool.

Two correctness hazards exist:

1. workers can finish records from the same partition out of order; and
2. a consumer-group rebalance can move partition ownership while workers are
   still processing a polled batch.

A simple `CommitRecords` call from each worker is therefore unsafe. A later
offset can be committed while an earlier record is unfinished, or a worker can
attempt to commit after the partition has moved to another group member.

## Decision

TelemetryForge applies two coordination rules.

### 1. Contiguous per-partition commits

Records are registered in poll order. Completion marks one registered offset as
done, but the Kafka position advances only through the highest **contiguous**
completed prefix.

Different partitions have independent acknowledgement state.

### 2. Rebalances are blocked for one bounded poll batch

The Kafka client uses franz-go `BlockRebalanceOnPoll`.

`PollRecords` returns at most 100 records. Every submitted worker job signals a
completion callback. The consumer waits until all submitted jobs from that
batch finish before calling `AllowRebalance`.

`CloseAllowingRebalance` is used during shutdown.

## Alternatives considered

### Commit each record when its worker finishes

Simple, but unsafe because Kafka offsets are partition positions rather than
independent record acknowledgements.

### Process one record at a time

Correct but discards most useful worker concurrency.

### Process one partition serially

Preserves ordering but complicates scheduling and can underuse CPU when one
partition contains slow work.

### Allow rebalances during asynchronous processing

Higher rebalance responsiveness, but commits can race partition revocation.

## Consequences

TelemetryForge preserves concurrent processing while preventing commits from
passing unfinished same-partition work.

Blocking rebalances introduces a new operational requirement: bounded batch
processing must remain below the consumer-group rebalance timeout. Poll size and
local retries are bounded specifically to make that assumption manageable.

v0.7.0 scaling/observability work should expose lag and processing duration so
operators can detect when this assumption is becoming unsafe.
