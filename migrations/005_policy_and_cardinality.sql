-- TelemetryForge v0.8.0 Cardinality Firewall and shadow-policy evidence.

CREATE TABLE IF NOT EXISTS cardinality_findings (
    finding_id BIGSERIAL PRIMARY KEY,
    observed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    policy_name TEXT NOT NULL,
    policy_version TEXT NOT NULL,
    mode TEXT NOT NULL CHECK (mode IN ('active', 'shadow')),
    source TEXT NOT NULL,
    event_type TEXT NOT NULL,
    dimension TEXT NOT NULL,
    observed_unique BIGINT NOT NULL,
    projected_unique BIGINT NOT NULL,
    action TEXT NOT NULL,
    reason TEXT NOT NULL,
    value_fingerprint TEXT NOT NULL,
    first_seen TIMESTAMPTZ NOT NULL,
    last_seen TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS cardinality_findings_observed_idx
    ON cardinality_findings (observed_at DESC);

CREATE INDEX IF NOT EXISTS cardinality_findings_dimension_idx
    ON cardinality_findings (source, event_type, dimension, observed_at DESC);

CREATE TABLE IF NOT EXISTS policy_shadow_diffs (
    diff_id BIGSERIAL PRIMARY KEY,
    observed_at TIMESTAMPTZ NOT NULL,
    source TEXT NOT NULL,
    event_type TEXT NOT NULL,
    dimension TEXT NOT NULL,
    active_policy TEXT NOT NULL,
    active_version TEXT NOT NULL,
    active_action TEXT NOT NULL,
    shadow_policy TEXT NOT NULL,
    shadow_version TEXT NOT NULL,
    shadow_action TEXT NOT NULL,
    reason TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS policy_shadow_diffs_observed_idx
    ON policy_shadow_diffs (observed_at DESC);

CREATE TABLE IF NOT EXISTS quarantined_events (
    event_id TEXT PRIMARY KEY,
    quarantined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    source TEXT NOT NULL,
    event_type TEXT NOT NULL,
    reason TEXT NOT NULL,
    envelope JSONB NOT NULL
);

CREATE INDEX IF NOT EXISTS quarantined_events_time_idx
    ON quarantined_events (quarantined_at DESC);

-- Findings/diffs are ordinary relational operational evidence in v0.8.0.
-- Automated retention is intentionally deferred until their investigation
-- lifecycle is measured rather than applying a Timescale policy to normal tables.
