-- TelemetryForge v1.2.0 distributed cardinality intelligence.
--
-- Shared state stores only fixed-size HLL registers and up to 16 hashed values
-- for low-cardinality exactness. Raw tag values are never persisted here.

CREATE TABLE IF NOT EXISTS cardinality_cluster_state (
    tenant_id TEXT NOT NULL,
    mode TEXT NOT NULL CHECK (mode IN ('active', 'shadow')),
    source TEXT NOT NULL,
    event_type TEXT NOT NULL,
    dimension TEXT NOT NULL,
    window_start TIMESTAMPTZ NOT NULL,
    registers BYTEA NOT NULL DEFAULT decode(repeat('00', 64), 'hex'),
    exact_hashes BIGINT[] NOT NULL DEFAULT '{}',
    exact_overflow BOOLEAN NOT NULL DEFAULT FALSE,
    first_seen TIMESTAMPTZ NOT NULL,
    last_seen TIMESTAMPTZ NOT NULL,
    samples BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (tenant_id, mode, source, event_type, dimension, window_start),
    CHECK (octet_length(registers) = 64),
    CHECK (cardinality(exact_hashes) <= 16)
);

SELECT create_hypertable(
    'cardinality_cluster_state',
    by_range('window_start', INTERVAL '1 day'),
    if_not_exists => TRUE
);

CREATE INDEX IF NOT EXISTS cardinality_cluster_state_lookup_idx
    ON cardinality_cluster_state
       (tenant_id, mode, window_start DESC, source, event_type, dimension);

SELECT add_retention_policy(
    'cardinality_cluster_state',
    INTERVAL '7 days',
    if_not_exists => TRUE
);

CREATE TABLE IF NOT EXISTS cardinality_budget_status (
    tenant_id TEXT NOT NULL,
    policy_name TEXT NOT NULL,
    policy_version TEXT NOT NULL,
    mode TEXT NOT NULL CHECK (mode IN ('active', 'shadow')),
    budget_name TEXT NOT NULL,
    source_pattern TEXT NOT NULL,
    event_type_pattern TEXT NOT NULL,
    window_start TIMESTAMPTZ NOT NULL,
    series_limit BIGINT NOT NULL,
    observed_unique BIGINT NOT NULL,
    projected_unique BIGINT NOT NULL,
    consumption_percent DOUBLE PRECISION NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('healthy', 'warning', 'critical', 'exceeded')),
    observed_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (
        tenant_id, policy_name, policy_version, mode, budget_name
    )
);

CREATE INDEX IF NOT EXISTS cardinality_budget_status_tenant_idx
    ON cardinality_budget_status (tenant_id, mode, status, observed_at DESC);
