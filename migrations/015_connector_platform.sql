-- TelemetryForge v1.8.0 Connector Platform runtime metadata.
CREATE TABLE IF NOT EXISTS connector_runtime_state (
    instance_id TEXT NOT NULL,
    destination TEXT NOT NULL,
    connector_kind TEXT NOT NULL,
    protocol TEXT NOT NULL,
    signals JSONB NOT NULL DEFAULT '[]'::jsonb,
    health_check BOOLEAN NOT NULL,
    retry_classification BOOLEAN NOT NULL,
    ready BOOLEAN NOT NULL,
    last_error TEXT NOT NULL DEFAULT '',
    checked_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (instance_id, destination)
);
CREATE INDEX IF NOT EXISTS connector_runtime_state_checked_idx ON connector_runtime_state (checked_at DESC);
CREATE INDEX IF NOT EXISTS connector_runtime_state_destination_idx ON connector_runtime_state (destination, checked_at DESC);
