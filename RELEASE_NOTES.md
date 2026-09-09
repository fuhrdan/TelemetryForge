# TelemetryForge v0.4.0 Release Notes

## Durable Time-Series Persistence

v0.4.0 converts the processing pipeline into a durable observability data path.

### Added

- PostgreSQL/TimescaleDB persistence.
- TimescaleDB hypertable for telemetry events.
- PostgreSQL global event-ID deduplication table.
- Transactional idempotent storage.
- pgx connection pooling.
- JSONB tags and payloads.
- source/time, type/time, correlation/time, and tag indexes.
- 30-day development retention policy.
- `GET /api/v1/events`.
- `GET /api/v1/metrics`.
- bounded RFC3339 time-range queries.
- persistence processor and processor chain.
- TimescaleDB Compose service.
- human-readable storage schema, retention, and query documentation.
- ADRs for TimescaleDB and deduplication design.

### Reliability contract

A worker now follows this sequence:

1. consume Kafka record;
2. normalize event;
3. begin PostgreSQL transaction;
4. reserve canonical event ID;
5. insert time-series event if new;
6. commit PostgreSQL transaction;
7. commit Kafka offset.

Database failure therefore prevents Kafka acknowledgement.

A repeated event ID is treated as successful, already-completed work, making
Kafka retry behavior idempotent at the persistence boundary.

### Known limitations

- Query API does not yet paginate with cursors.
- Aggregated/downsampled query endpoints are not implemented.
- Deduplication metadata retention is not yet automated.
- DLQ/retry ceilings arrive in v0.5.0.
- No authentication or tenant authorization exists yet.
- Local Compose credentials are development-only.

These limitations are documented intentionally.
