-- TelemetryForge v0.9.0 incident replay and cost-simulation history.

CREATE TABLE IF NOT EXISTS replay_runs (
    run_id TEXT PRIMARY KEY,
    incident_id TEXT NOT NULL,
    mode TEXT NOT NULL,
    status TEXT NOT NULL,
    active_policy TEXT NOT NULL,
    active_version TEXT NOT NULL,
    shadow_policy TEXT,
    shadow_version TEXT,
    output_topic TEXT,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    event_count BIGINT NOT NULL DEFAULT 0,
    changed_event_count BIGINT NOT NULL DEFAULT 0,
    dropped_tag_count BIGINT NOT NULL DEFAULT 0,
    quarantined_count BIGINT NOT NULL DEFAULT 0,
    finding_count BIGINT NOT NULL DEFAULT 0,
    shadow_diff_count BIGINT NOT NULL DEFAULT 0,
    published_count BIGINT NOT NULL DEFAULT 0,
    error TEXT
);

CREATE INDEX IF NOT EXISTS replay_runs_started_idx
    ON replay_runs (started_at DESC);

CREATE TABLE IF NOT EXISTS replay_event_results (
    run_id TEXT NOT NULL REFERENCES replay_runs(run_id) ON DELETE CASCADE,
    event_id TEXT NOT NULL,
    changed BOOLEAN NOT NULL,
    dropped_tag_count INTEGER NOT NULL,
    quarantined BOOLEAN NOT NULL,
    finding_count INTEGER NOT NULL,
    shadow_diff_count INTEGER NOT NULL,
    PRIMARY KEY (run_id, event_id)
);

CREATE TABLE IF NOT EXISTS cost_simulations (
    simulation_id TEXT PRIMARY KEY,
    incident_id TEXT NOT NULL,
    active_policy TEXT NOT NULL,
    active_version TEXT NOT NULL,
    shadow_policy TEXT,
    shadow_version TEXT,
    pricing_model TEXT,
    currency TEXT,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    window_seconds DOUBLE PRECISION NOT NULL DEFAULT 0,
    event_count BIGINT NOT NULL DEFAULT 0,
    baseline_bytes BIGINT NOT NULL DEFAULT 0,
    active_bytes BIGINT NOT NULL DEFAULT 0,
    shadow_bytes BIGINT NOT NULL DEFAULT 0,
    baseline_series BIGINT NOT NULL DEFAULT 0,
    active_series BIGINT NOT NULL DEFAULT 0,
    shadow_series BIGINT NOT NULL DEFAULT 0,
    projected_monthly_baseline_gb DOUBLE PRECISION NOT NULL DEFAULT 0,
    projected_monthly_active_gb DOUBLE PRECISION NOT NULL DEFAULT 0,
    projected_monthly_shadow_gb DOUBLE PRECISION NOT NULL DEFAULT 0,
    projected_monthly_baseline_cost DOUBLE PRECISION,
    projected_monthly_active_cost DOUBLE PRECISION,
    projected_monthly_shadow_cost DOUBLE PRECISION,
    active_changed_events BIGINT NOT NULL DEFAULT 0,
    shadow_changed_events BIGINT NOT NULL DEFAULT 0,
    assumptions JSONB NOT NULL DEFAULT '{}'::jsonb,
    error TEXT
);

CREATE INDEX IF NOT EXISTS cost_simulations_started_idx
    ON cost_simulations (started_at DESC);
