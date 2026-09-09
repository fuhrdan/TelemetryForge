-- TelemetryForge v0.5.0 reliability foundation.
--
-- The flight recorder deliberately keeps a short, full-fidelity rolling
-- buffer. "Freezing" an incident copies a selected window out of that rolling
-- buffer before retention removes it.

CREATE TABLE IF NOT EXISTS flight_recorder_events (
    event_id TEXT NOT NULL,
    captured_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    event_time TIMESTAMPTZ NOT NULL,
    source TEXT NOT NULL,
    event_type TEXT NOT NULL,
    envelope JSONB NOT NULL
);

SELECT create_hypertable(
    'flight_recorder_events',
    by_range('captured_at', INTERVAL '5 minutes'),
    if_not_exists => TRUE
);

CREATE INDEX IF NOT EXISTS flight_recorder_source_capture_idx
    ON flight_recorder_events (source, captured_at DESC);

CREATE INDEX IF NOT EXISTS flight_recorder_event_time_idx
    ON flight_recorder_events (event_time DESC);

-- Short by design: this is a rolling "black box," not the main datastore.
SELECT add_retention_policy(
    'flight_recorder_events',
    INTERVAL '30 minutes',
    if_not_exists => TRUE
);

CREATE TABLE IF NOT EXISTS incidents (
    incident_id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    frozen_from TIMESTAMPTZ NOT NULL,
    frozen_to TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS incident_events (
    incident_id TEXT NOT NULL REFERENCES incidents(incident_id) ON DELETE CASCADE,
    event_id TEXT NOT NULL,
    captured_at TIMESTAMPTZ NOT NULL,
    event_time TIMESTAMPTZ NOT NULL,
    source TEXT NOT NULL,
    event_type TEXT NOT NULL,
    envelope JSONB NOT NULL,
    PRIMARY KEY (incident_id, event_id, captured_at)
);

CREATE INDEX IF NOT EXISTS incident_events_time_idx
    ON incident_events (incident_id, event_time);
