# Worker Model

The worker is responsible for five distinct behaviors:

1. keep application memory bounded;
2. preserve short-lived full-fidelity incident evidence;
3. retry only failures that are classified as transient;
4. persist an event or durably replace it with a DLQ record; and
5. coordinate Kafka acknowledgements safely across concurrent workers.

## Processing path

```text
Kafka poll (max 100 records)
       |
       | rebalance blocked for this batch
       v
bounded worker queue
       |
       v
Flight Recorder
       |
       v
Normalizer
       |
       v
TimescaleDB persistence
       |
       v
Incident detector
       |
       +---- success ------------------+
       |                               |
       `---- failure                   |
               |                       |
               +-- transient --> retry |
               |             exhausted |
               +-----------------------+
               |
               v
         telemetry.dlq
               |
               v
      source record handled
               |
               v
contiguous partition acknowledgement
               |
               v
all submitted poll jobs complete
               |
               v
allow Kafka rebalance
```

## Why a bounded queue?

Kafka is the durable backlog. An unbounded Go queue would merely copy that
backlog into process memory and eventually turn downstream slowness into an
out-of-memory failure.

When the queue fills, `Submit` waits and Kafka lag grows instead.

## Why record before normalization?

The Flight Recorder exists to preserve evidence. It should contain what the
processing pipeline received, not only what later processing decided to keep or
change.

## Why retry inside a worker slot?

A retry remains attached to the worker that owns the job. That keeps retry
concurrency bounded by worker count. TelemetryForge does not launch a new
goroutine or create a second in-memory retry queue for every failed attempt.

## Why does DLQ success count as handled?

Once Kafka durably contains a detailed dead-letter replacement, the poison
record no longer needs to block the source partition. The DLQ preserves the
event, source location, failure reason, and attempt count for investigation and
replay.

If publishing that replacement fails, the original source record remains
uncommitted.

## Why not commit every worker result immediately?

Workers can finish records from one partition out of order.

A later Kafka offset is not safe to commit while an earlier offset is still
processing. The acknowledgement coordinator therefore advances each partition
only through its contiguous completed prefix.

## Why block rebalances for a poll batch?

A Kafka group can otherwise reassign a partition while asynchronous jobs from
that partition are still running.

TelemetryForge uses a bounded `PollRecords` batch and franz-go
`BlockRebalanceOnPoll`. The consumer releases the rebalance gate only after
every submitted job from that poll has completed its normal or DLQ handling.

This keeps commits within the ownership epoch that produced the work.

See `docs/kafka/delivery-semantics.md` and ADR 0013 for the full trade-off.
