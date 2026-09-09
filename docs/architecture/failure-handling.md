# Failure Handling in v0.4.0

v0.4.0 extends the at-least-once processing contract through durable storage.

## Kafka publish failure

The gateway returns `503 Service Unavailable`. It does not claim that an event
was accepted when Kafka did not acknowledge it.

## Processing failure

If normalization or another processor fails, the worker does not commit the
Kafka record.

## Database failure

The worker transaction fails and the Kafka record remains uncommitted. The
record can be delivered again after recovery.

## Duplicate delivery

The database transaction first inserts the event ID into `event_dedup` using
`ON CONFLICT DO NOTHING`. If that ID already exists, persistence returns
success without creating a second time-series row.

This is the key bridge between Kafka's at-least-once behavior and safe durable
storage.

## Malformed Kafka record

Malformed JSON is logged with topic, partition, and offset. Automated
dead-letter handling is intentionally deferred to v0.5.0 so retry classification,
retry ceilings, DLQ metadata, and replay semantics are implemented as one
coherent reliability feature.

## Shutdown

SIGINT/SIGTERM stops polling for new work. Jobs already in the bounded queue
finish, successful database transactions are acknowledged, and then Kafka and
database connections close.
