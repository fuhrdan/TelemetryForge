-- TelemetryForge v2.0.0 evidence-first intelligence snapshots.
--
-- Intelligence output is deterministic/advisory. It stores cited synthesis
-- results for audit/history; it never mutates lifecycle/configuration state.
CREATE TABLE IF NOT EXISTS intelligence_snapshots (
    tenant_id TEXT NOT NULL,
    incident_id TEXT NOT NULL,
    generated_at TIMESTAMPTZ NOT NULL,
    status TEXT NOT NULL,
    investigation JSONB NOT NULL,
    PRIMARY KEY (tenant_id, incident_id)
);

CREATE INDEX IF NOT EXISTS intelligence_snapshots_generated_idx
    ON intelligence_snapshots (tenant_id, generated_at DESC);
