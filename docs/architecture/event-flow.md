# Event Flow — v0.6.0

## Normal telemetry

1. Client submits telemetry.
2. Gateway validates the canonical envelope.
3. Kafka acknowledges the record.
4. Gateway returns HTTP `202`.
5. Worker-group member consumes the record.
6. Flight Recorder stores the untouched decoded envelope.
7. Normalizer canonicalizes source/type.
8. TimescaleDB persistence commits idempotently.
9. Automatic incident rules evaluate the durable event.
10. Kafka source offset commits.

## Live dashboard

1. Browser opens `GET /api/v1/live` through the Next.js proxy.
2. Go gateway polls the durable store once per second.
3. New records are read using `(ingested_at, event_id)` as a stable cursor.
4. Records are emitted in chronological order as SSE `telemetry` events.
5. Browser updates the live table/chart without a full page refresh.

The tuple cursor prevents equal ingestion timestamps from being skipped and
avoids losing newly arrived telemetry whose source `event_time` is old or out
of order. SSE IDs also carry this cursor so browser reconnects can resume from
`Last-Event-ID`.

## Automatic incident

After persistence, the detector evaluates latency and error-burst rules.

On a breach:

1. cooldown is checked;
2. an automatic incident ID is created;
3. the previous 15 minutes of Flight Recorder data are copied to
   `incident_events`;
4. the trigger reason is recorded;
5. the incident appears in the dashboard's periodic refresh.

Incident-freeze failure is logged but does not undo or DLQ the already durable
telemetry event.

## Terminal processing failure

Permanent or retry-exhausted processing failures are published to
`telemetry.dlq`. The original source offset commits only after Kafka
acknowledges the dead-letter replacement.
