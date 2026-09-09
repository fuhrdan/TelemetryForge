# Event Flow — v0.5.0

## Successful path

1. Client posts telemetry to the gateway.
2. Gateway validates the canonical envelope.
3. Kafka acknowledges the record.
4. Gateway returns `202 Accepted`.
5. A worker-group member consumes the record.
6. The full incoming envelope enters the rolling Flight Recorder.
7. Processing normalizes the event.
8. The database transaction reserves the event ID and writes the time-series row.
9. PostgreSQL commits.
10. The worker commits the Kafka source offset.

## Transient-failure path

1. A dependency operation fails with a transient classification.
2. The worker waits using exponential backoff with jitter.
3. Processing retries, up to four total attempts.
4. If a retry succeeds, the normal path continues.
5. If attempts are exhausted, the record enters the DLQ path.

## Permanent-failure path

1. Processing reports a permanent failure, or Kafka payload decoding fails.
2. TelemetryForge builds a dead-letter envelope containing source metadata.
3. Kafka acknowledges `telemetry.dlq`.
4. Only then is the original source offset committed.

## Incident path

Flight Recorder rows expire after 30 minutes by default. An operator can freeze
a capture window into durable `incident_events` before that happens.

These paths make loss, retries, duplicates, and terminal failures explicit
rather than hiding them behind application logs.
