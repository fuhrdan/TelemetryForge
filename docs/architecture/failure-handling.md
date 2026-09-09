# Failure Handling in v0.5.0

v0.5.0 makes retries and dead-letter behavior explicit.

## Gateway publish failure

The gateway returns `503 Service Unavailable` and does not claim durable
acceptance unless Kafka acknowledges the ingest record.

## Processing failure classes

A processing error is either:

- **transient** — retrying may succeed; or
- **permanent** — the same input is not expected to improve.

Unknown errors default to permanent.

## Transient failure

The worker retries with bounded exponential backoff and jitter. Four total
attempts are allowed by default.

If all attempts fail, the event moves to `telemetry.dlq`.

## Permanent failure

The worker skips local retry and sends the event directly to the DLQ.

## DLQ failure

The original Kafka record is committed only after Kafka acknowledges the DLQ
record. If DLQ publication fails, the original offset remains uncommitted.

## Malformed Kafka data

Records that cannot be decoded preserve their original bytes in the DLQ using
base64 JSON encoding, along with topic, partition, offset, and decode error.

## Database / Flight Recorder failure

Both are classified as transient dependency failures. If they remain
unavailable through the retry ceiling, the source event enters the DLQ.

## Shutdown

Cancellation stops polling for new work. In-flight work either completes or
remains uncommitted if the shutdown context prevents persistence/DLQ
acknowledgement.

This preserves recoverability over falsely claiming completion.
