-- TelemetryForge v1.6.0 immutable configuration lifecycle and runtime convergence.
CREATE TABLE IF NOT EXISTS configuration_artifacts (
    artifact_id TEXT PRIMARY KEY,
    kind TEXT NOT NULL CHECK (kind IN ('policy','shaping','routing')),
    name TEXT NOT NULL,
    version TEXT NOT NULL,
    sha256 TEXT NOT NULL,
    payload JSONB NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('draft','shadow','approved','scheduled','active','retired')),
    created_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    scheduled_for TIMESTAMPTZ,
    activated_at TIMESTAMPTZ,
    retired_at TIMESTAMPTZ,
    UNIQUE (kind, name, version),
    UNIQUE (sha256)
);
CREATE UNIQUE INDEX IF NOT EXISTS configuration_artifacts_one_active_kind_idx
    ON configuration_artifacts (kind) WHERE state='active';
CREATE UNIQUE INDEX IF NOT EXISTS configuration_artifacts_one_pending_kind_idx
    ON configuration_artifacts (kind) WHERE state IN ('shadow','approved','scheduled');

CREATE TABLE IF NOT EXISTS configuration_approvals (
    artifact_id TEXT PRIMARY KEY REFERENCES configuration_artifacts(artifact_id) ON DELETE CASCADE,
    actor TEXT NOT NULL,
    comment TEXT NOT NULL DEFAULT '',
    approved_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS configuration_evidence (
    evidence_id TEXT PRIMARY KEY,
    artifact_id TEXT NOT NULL REFERENCES configuration_artifacts(artifact_id) ON DELETE CASCADE,
    evidence_type TEXT NOT NULL,
    reference TEXT NOT NULL,
    result TEXT NOT NULL CHECK (result IN ('pass','fail','waived')),
    reason TEXT NOT NULL DEFAULT '',
    tenant_id TEXT NOT NULL DEFAULT '',
    recorded_by TEXT NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS configuration_evidence_artifact_idx ON configuration_evidence(artifact_id, recorded_at DESC);

CREATE TABLE IF NOT EXISTS configuration_audit (
    audit_id BIGSERIAL PRIMARY KEY,
    artifact_id TEXT,
    kind TEXT,
    action TEXT NOT NULL,
    actor TEXT NOT NULL,
    detail TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS configuration_audit_created_idx ON configuration_audit(created_at DESC);

CREATE TABLE IF NOT EXISTS configuration_runtime_loads (
    instance_id TEXT NOT NULL,
    component TEXT NOT NULL,
    kind TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('active','shadow')),
    artifact_id TEXT NOT NULL,
    sha256 TEXT NOT NULL,
    loaded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY(instance_id, component, kind, role)
);
CREATE INDEX IF NOT EXISTS configuration_runtime_loads_time_idx ON configuration_runtime_loads(loaded_at DESC);
