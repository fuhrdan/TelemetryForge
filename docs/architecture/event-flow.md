# Event Flow

## Normal telemetry

1. Client submits telemetry.
2. Gateway validates the canonical envelope.
3. Kafka acknowledges the record.
4. Gateway returns HTTP `202`.
5. Worker-group member consumes the record.
6. Flight Recorder stores the untouched decoded envelope.
7. Normalizer canonicalizes source/type.
8. Active Cardinality Firewall policy evaluates/mutates the normal event.
9. Candidate shadow policy evaluates the same normalized event without mutation.
10. Policy findings/differences/quarantine evidence are persisted as needed.
11. TimescaleDB primary telemetry persistence commits idempotently.
12. Automatic incident rules evaluate the durable event.
13. Worker completion marks the source record handled.
14. The acknowledgement coordinator advances the partition only through its
    contiguous completed prefix.
15. After every submitted job from the bounded poll batch finishes, the
    consumer allows a group rebalance.

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


## Cardinality policy path

For every tag dimension:

1. bounded HyperLogLog state observes a SHA-256-derived hash;
2. active policy selects threshold/action;
3. a finding is rate-limited and stored when risk is detected;
4. `drop_tag` removes the dimension from the normal event;
5. `quarantine` preserves a full normalized copy separately and removes the
   risky dimension from the normal representation;
6. shadow policy evaluates independently;
7. active/shadow differences are stored for review.

The raw risky tag value is not copied into the cardinality-finding table.


## v1.1 schema observation

After normalization, the worker derives a bounded schema description and writes
an idempotent registry observation before active/shadow policy.

A drift finding is evidence only; it does not reject the event. If registry
storage is unavailable and `TELEMETRYFORGE_SCHEMA_FAIL_OPEN=true`, the event
continues to policy/persistence.
