-- TelemetryForge v1.1.0 Schema Intelligence.
--
-- Schema observations are idempotent by tenant/event ID so worker retry cannot
-- inflate counts or manufacture drift. Registry rows are versioned by the
-- client-declared schema_version already present in the canonical envelope.

CREATE TABLE IF NOT EXISTS schema_observations (
    tenant_id TEXT NOT NULL,
    event_id TEXT NOT NULL,
    source TEXT NOT NULL,
    event_type TEXT NOT NULL,
    declared_version TEXT NOT NULL,
    schema_url TEXT NOT NULL DEFAULT '',
    fingerprint TEXT NOT NULL,
    observed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, event_id)
);

CREATE INDEX IF NOT EXISTS schema_observations_identity_idx
    ON schema_observations (tenant_id, source, event_type, declared_version, observed_at DESC);

CREATE INDEX IF NOT EXISTS schema_observations_tenant_time_idx
    ON schema_observations (tenant_id, observed_at);

CREATE TABLE IF NOT EXISTS schema_registry (
    tenant_id TEXT NOT NULL,
    source TEXT NOT NULL,
    event_type TEXT NOT NULL,
    declared_version TEXT NOT NULL,
    schema_url TEXT NOT NULL DEFAULT '',
    fingerprint TEXT NOT NULL,
    health TEXT NOT NULL DEFAULT 'healthy',
    first_seen TIMESTAMPTZ NOT NULL,
    last_seen TIMESTAMPTZ NOT NULL,
    observation_count BIGINT NOT NULL DEFAULT 0,
    field_count INTEGER NOT NULL DEFAULT 0,
    required_field_count INTEGER NOT NULL DEFAULT 0,
    fields JSONB NOT NULL DEFAULT '[]'::jsonb,
    semantic_findings JSONB NOT NULL DEFAULT '[]'::jsonb,
    PRIMARY KEY (tenant_id, source, event_type, declared_version)
);

CREATE INDEX IF NOT EXISTS schema_registry_tenant_health_idx
    ON schema_registry (tenant_id, health, last_seen DESC);

CREATE INDEX IF NOT EXISTS schema_registry_tenant_identity_idx
    ON schema_registry (tenant_id, source, event_type, last_seen DESC);

CREATE TABLE IF NOT EXISTS schema_drift_findings (
    tenant_id TEXT NOT NULL,
    source TEXT NOT NULL,
    event_type TEXT NOT NULL,
    declared_version TEXT NOT NULL,
    signature TEXT NOT NULL,
    severity TEXT NOT NULL,
    kind TEXT NOT NULL,
    path TEXT NOT NULL DEFAULT '',
    previous_type TEXT NOT NULL DEFAULT '',
    current_type TEXT NOT NULL DEFAULT '',
    message TEXT NOT NULL,
    first_seen TIMESTAMPTZ NOT NULL,
    last_seen TIMESTAMPTZ NOT NULL,
    occurrences BIGINT NOT NULL DEFAULT 1,
    PRIMARY KEY (tenant_id, source, event_type, declared_version, signature)
);

CREATE INDEX IF NOT EXISTS schema_drift_tenant_recent_idx
    ON schema_drift_findings (tenant_id, last_seen DESC);

CREATE INDEX IF NOT EXISTS schema_drift_tenant_severity_idx
    ON schema_drift_findings (tenant_id, severity, last_seen DESC);
