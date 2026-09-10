-- TelemetryForge v1.7.0 Change Intelligence.
--
-- change_markers stores normalized deployment/release/rollback/change evidence
-- before lossy shaping. change_analysis_snapshots stores regenerable bounded
-- before/after analysis artifacts; primary telemetry remains the source of truth.

CREATE TABLE IF NOT EXISTS change_markers (
    tenant_id TEXT NOT NULL,
    change_id TEXT NOT NULL,
    event_id TEXT NOT NULL,
    source TEXT NOT NULL,
    kind TEXT NOT NULL,
    status TEXT NOT NULL,
    environment TEXT,
    version TEXT,
    previous_version TEXT,
    git_sha TEXT,
    build_id TEXT,
    actor TEXT,
    rollback_of TEXT,
    summary TEXT,
    changed_at TIMESTAMPTZ NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, change_id),
    UNIQUE (tenant_id, event_id)
);

CREATE INDEX IF NOT EXISTS change_markers_tenant_time_idx
    ON change_markers (tenant_id, changed_at DESC);
CREATE INDEX IF NOT EXISTS change_markers_tenant_source_time_idx
    ON change_markers (tenant_id, source, changed_at DESC);
CREATE INDEX IF NOT EXISTS change_markers_tenant_rollback_idx
    ON change_markers (tenant_id, rollback_of, changed_at DESC)
    WHERE rollback_of IS NOT NULL;

CREATE TABLE IF NOT EXISTS change_analysis_snapshots (
    tenant_id TEXT NOT NULL,
    change_id TEXT NOT NULL,
    generated_at TIMESTAMPTZ NOT NULL,
    assessment TEXT NOT NULL,
    regressed_sources INTEGER NOT NULL,
    assessable_sources INTEGER NOT NULL,
    blast_radius_percent DOUBLE PRECISION NOT NULL,
    analysis JSONB NOT NULL,
    PRIMARY KEY (tenant_id, change_id),
    FOREIGN KEY (tenant_id, change_id)
      REFERENCES change_markers (tenant_id, change_id)
      ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS change_analysis_tenant_generated_idx
    ON change_analysis_snapshots (tenant_id, generated_at DESC);
