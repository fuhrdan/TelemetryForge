-- TelemetryForge v1.5.0 portable incident archive provenance.
--
-- Imported archives are audit-sensitive operational actions. The portable
-- incident remains a file; this table records where imported incident evidence
-- came from without storing a second copy of the archive bytes.

CREATE TABLE IF NOT EXISTS incident_archive_imports (
    tenant_id TEXT NOT NULL,
    archive_id TEXT NOT NULL,
    source_tenant_id TEXT NOT NULL,
    source_incident_id TEXT NOT NULL,
    imported_incident_id TEXT NOT NULL,
    format_version INTEGER NOT NULL,
    telemetryforge_version TEXT NOT NULL,
    file_sha256 TEXT NOT NULL,
    encrypted BOOLEAN NOT NULL,
    imported_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, archive_id)
);

CREATE INDEX IF NOT EXISTS incident_archive_imports_incident_idx
    ON incident_archive_imports (tenant_id, imported_incident_id, imported_at DESC);
