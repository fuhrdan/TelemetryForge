# Storage Schema

v0.4.0 introduces durable telemetry storage using PostgreSQL plus TimescaleDB.

## Why both?

TimescaleDB is a PostgreSQL extension, so TelemetryForge gets ordinary
relational tables and time-series hypertables in one database.

The storage model deliberately separates two concerns:

### `event_dedup`

A normal PostgreSQL table:

```text
event_id        PRIMARY KEY
first_seen_at   timestamp
```

Its job is global idempotency. Kafka can deliver a record more than once under
at-least-once processing. Reserving the event ID here prevents a retry from
creating a second stored event.

### `telemetry_events`

A TimescaleDB hypertable partitioned by `event_time`.

It stores:

- event ID
- source
- event type
- original event timestamp
- JSONB tags
- JSONB payload
- optional metric value/unit
- schema version
- correlation ID
- ingestion timestamp

## Why not put a unique constraint on `event_id` in the hypertable?

TimescaleDB unique constraints on a hypertable must include the partitioning
column. A unique `(event_id, event_time)` constraint would not protect against a
retry that somehow carried the same ID with a different timestamp.

The dedicated `event_dedup` table gives event IDs true global uniqueness.

## Transaction boundary

The worker performs these operations in one PostgreSQL transaction:

```text
INSERT event_dedup(event_id)
        |
        +-- already exists --> safe no-op
        |
        `-- newly inserted
                |
                v
        INSERT telemetry_events
                |
                v
              COMMIT
```

Only after that transaction succeeds does the worker commit the Kafka offset.

## Indexes

v0.4.0 creates indexes for the expected first dashboard/query patterns:

- `(source, event_time DESC)`
- `(event_type, event_time DESC)`
- `(correlation_id, event_time DESC)` for correlated events
- GIN index on `tags`

Indexes should be added from measured query behavior rather than creating an
index for every possible field.
