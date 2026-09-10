-- TelemetryForge v1.4.0 adaptive sampling / shaping evidence.
--
-- A compact per-event decision ledger makes adaptive decisions stable across
-- worker retries. It stores no telemetry payload, only the policy decision and
-- transformation metadata required to reproduce the shaped result.
--
-- Minute aggregates are incremented only when a decision is inserted for the
-- first time, preventing whole-chain retries from inflating statistics.

CREATE TABLE IF NOT EXISTS shaping_decisions (
    tenant_id TEXT NOT NULL,
    event_id TEXT NOT NULL,
    config_name TEXT NOT NULL,
    config_version TEXT NOT NULL,
    source TEXT NOT NULL,
    event_type TEXT NOT NULL,
    observed_at TIMESTAMPTZ NOT NULL,
    rules TEXT[] NOT NULL DEFAULT '{}',
    keep BOOLEAN NOT NULL,
    protected BOOLEAN NOT NULL,
    protection_reason TEXT NOT NULL DEFAULT '',
    base_rate DOUBLE PRECISION NOT NULL,
    effective_rate DOUBLE PRECISION NOT NULL,
    queue_pressure DOUBLE PRECISION NOT NULL,
    dropped_tags TEXT[] NOT NULL DEFAULT '{}',
    renamed_tags JSONB NOT NULL DEFAULT '{}',
    payload_dropped BOOLEAN NOT NULL,
    original_bytes BIGINT NOT NULL,
    shaped_bytes BIGINT NOT NULL,
    reason TEXT NOT NULL,
    PRIMARY KEY (tenant_id,event_id,config_name,config_version)
);

CREATE INDEX IF NOT EXISTS shaping_decisions_tenant_observed_idx
    ON shaping_decisions (tenant_id,observed_at DESC);

CREATE TABLE IF NOT EXISTS shaping_minute_stats (
    tenant_id TEXT NOT NULL,
    bucket TIMESTAMPTZ NOT NULL,
    config_name TEXT NOT NULL,
    config_version TEXT NOT NULL,
    rule_name TEXT NOT NULL,
    source TEXT NOT NULL,
    event_type TEXT NOT NULL,
    observed BIGINT NOT NULL DEFAULT 0,
    kept BIGINT NOT NULL DEFAULT 0,
    sampled_out BIGINT NOT NULL DEFAULT 0,
    protected BIGINT NOT NULL DEFAULT 0,
    transformed BIGINT NOT NULL DEFAULT 0,
    payload_dropped BIGINT NOT NULL DEFAULT 0,
    original_bytes BIGINT NOT NULL DEFAULT 0,
    shaped_bytes BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id,bucket,config_name,config_version,rule_name,source,event_type)
);

CREATE INDEX IF NOT EXISTS shaping_minute_stats_tenant_bucket_idx
    ON shaping_minute_stats (tenant_id,bucket DESC);

CREATE TABLE IF NOT EXISTS shaping_shadow_diffs (
    tenant_id TEXT NOT NULL,
    event_id TEXT NOT NULL,
    observed_at TIMESTAMPTZ NOT NULL,
    active_config TEXT NOT NULL,
    active_version TEXT NOT NULL,
    shadow_config TEXT NOT NULL,
    shadow_version TEXT NOT NULL,
    active_keep BOOLEAN NOT NULL,
    shadow_keep BOOLEAN NOT NULL,
    active_rate DOUBLE PRECISION NOT NULL,
    shadow_rate DOUBLE PRECISION NOT NULL,
    active_effects TEXT[] NOT NULL DEFAULT '{}',
    shadow_effects TEXT[] NOT NULL DEFAULT '{}',
    PRIMARY KEY (
        tenant_id,event_id,active_config,active_version,shadow_config,shadow_version
    )
);

CREATE INDEX IF NOT EXISTS shaping_shadow_diffs_tenant_observed_idx
    ON shaping_shadow_diffs (tenant_id,observed_at DESC);
