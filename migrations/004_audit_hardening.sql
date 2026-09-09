-- TelemetryForge v0.7.0 development audit hardening.
--
-- These indexes support existing operational paths discovered during the
-- pre-v0.7 repository review. They do not introduce Cardinality Firewall or
-- Kubernetes features.

-- SSE live delivery orders by server ingestion time plus event ID. Without an
-- index, each one-second poll becomes increasingly expensive as retention grows.
CREATE INDEX IF NOT EXISTS telemetry_events_ingested_id_idx
    ON telemetry_events (ingested_at, event_id);

-- `telemetryctl dedup prune` deletes by first_seen_at.
CREATE INDEX IF NOT EXISTS event_dedup_first_seen_idx
    ON event_dedup (first_seen_at);
