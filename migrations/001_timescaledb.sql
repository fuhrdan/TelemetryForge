-- TelemetryForge v0.4.0 storage foundation.
--
-- PostgreSQL holds global idempotency metadata. TimescaleDB stores the
-- timestamp-partitioned telemetry stream.

CREATE EXTENSION IF NOT EXISTS timescaledb;

CREATE TABLE IF NOT EXISTS event_dedup (
    event_id TEXT PRIMARY KEY,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS telemetry_events (
    event_id TEXT NOT NULL,
    source TEXT NOT NULL,
    event_type TEXT NOT NULL,
    event_time TIMESTAMPTZ NOT NULL,
    tags JSONB NOT NULL DEFAULT '{}'::jsonb,
    payload JSONB,
    metric_value DOUBLE PRECISION,
    metric_unit TEXT,
    schema_version TEXT NOT NULL,
    correlation_id TEXT,
    ingested_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

SELECT create_hypertable(
    'telemetry_events',
    by_range('event_time'),
    if_not_exists => TRUE
);

-- Common dashboard/query access patterns.
CREATE INDEX IF NOT EXISTS telemetry_events_source_time_idx
    ON telemetry_events (source, event_time DESC);

CREATE INDEX IF NOT EXISTS telemetry_events_type_time_idx
    ON telemetry_events (event_type, event_time DESC);

CREATE INDEX IF NOT EXISTS telemetry_events_correlation_time_idx
    ON telemetry_events (correlation_id, event_time DESC)
    WHERE correlation_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS telemetry_events_tags_gin_idx
    ON telemetry_events USING GIN (tags);

-- v0.4.0 keeps 30 days by default. This is intentionally visible and easy to
-- change rather than hiding retention in application code.
SELECT add_retention_policy(
    'telemetry_events',
    INTERVAL '30 days',
    if_not_exists => TRUE
);
