# Failure Handling

TelemetryForge treats failures as explicit data-flow outcomes rather than
logging them and hoping the next component recovers.

## Gateway publish failure

The gateway returns `503 Service Unavailable` and does not claim durable
acceptance unless Kafka acknowledges the ingest record.

## Processing failure classes

A processing error is either:

- **transient** — retrying may succeed; or
- **permanent** — the same input is not expected to improve.

Unknown errors default to permanent so a programming defect cannot silently
become an unbounded retry loop.

## Transient failure

The worker retries with bounded exponential backoff and jitter. Four total
attempts are allowed by default.

If all attempts fail, the event moves to `telemetry.dlq`.

## Permanent failure

The worker skips local retry and sends the event directly to the DLQ.

## DLQ failure

The source Kafka record is not considered handled until Kafka acknowledges its
DLQ replacement.

If the DLQ publish fails:

1. the source record remains uncommitted;
2. the worker reports an unresolved record;
3. new polling is cancelled; and
4. already-submitted jobs drain before the current rebalance gate is released.

Restart/rebalance can then replay the unresolved work.

## Malformed Kafka data

Records that cannot be decoded preserve their original bytes in the DLQ using
base64 JSON encoding, along with topic, partition, offset, and decode error.

## Database / Flight Recorder failure

Both are transient dependency failures. If they remain unavailable through the
retry ceiling, the source event enters the DLQ path.

## Concurrent acknowledgement failure

A worker completion does not independently commit its Kafka offset.
Acknowledgements are coordinated per partition.

If a later record finishes first, it waits behind earlier unfinished work. A
commit advances only through the contiguous completed prefix.

## Consumer-group rebalance

Rebalances are blocked while one bounded poll batch is still processing. The
consumer waits for all jobs submitted from that batch, then calls
`AllowRebalance`.

This avoids committing after partition ownership has moved. It also means a
poll batch must remain comfortably below the Kafka rebalance timeout.

## Shutdown

Shutdown stops new polling and lets queued work drain. The consumer uses
rebalance-aware close behavior so a blocked rebalance cannot hang process exit.

The project favors recoverability and duplicate-safe replay over falsely
claiming completion.
