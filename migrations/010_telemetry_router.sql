-- TelemetryForge v1.3.0 durable multi-destination routing outbox.
--
-- The Kafka worker records delivery intent here and then acknowledges its
-- source record. Destination I/O happens in the separate router service, so a
-- failed backend cannot hold the primary processing partition open.

CREATE TABLE IF NOT EXISTS routing_deliveries (
    tenant_id TEXT NOT NULL,
    event_id TEXT NOT NULL,
    destination TEXT NOT NULL,
    route_rules TEXT[] NOT NULL DEFAULT '{}',
    envelope JSONB NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending','sending','retry','delivered','dead_letter')),
    attempts INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL,
    base_delay_ms INTEGER NOT NULL,
    max_delay_ms INTEGER NOT NULL,
    failure_fallback TEXT NOT NULL DEFAULT '',
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    lease_until TIMESTAMPTZ,
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    delivered_at TIMESTAMPTZ,
    PRIMARY KEY (tenant_id, event_id, destination),
    CHECK (attempts >= 0),
    CHECK (max_attempts >= 1),
    CHECK (base_delay_ms >= 1),
    CHECK (max_delay_ms >= base_delay_ms)
);

CREATE INDEX IF NOT EXISTS routing_deliveries_due_idx
    ON routing_deliveries (destination, status, next_attempt_at, created_at);

CREATE INDEX IF NOT EXISTS routing_deliveries_tenant_status_idx
    ON routing_deliveries (tenant_id, status, updated_at DESC);

CREATE TABLE IF NOT EXISTS routing_shadow_diffs (
    tenant_id TEXT NOT NULL,
    event_id TEXT NOT NULL,
    observed_at TIMESTAMPTZ NOT NULL,
    active_config TEXT NOT NULL,
    active_version TEXT NOT NULL,
    shadow_config TEXT NOT NULL,
    shadow_version TEXT NOT NULL,
    active_destinations TEXT[] NOT NULL DEFAULT '{}',
    shadow_destinations TEXT[] NOT NULL DEFAULT '{}',
    added TEXT[] NOT NULL DEFAULT '{}',
    removed TEXT[] NOT NULL DEFAULT '{}',
    PRIMARY KEY (tenant_id, event_id)
);

CREATE INDEX IF NOT EXISTS routing_shadow_diffs_tenant_time_idx
    ON routing_shadow_diffs (tenant_id, observed_at DESC);

CREATE TABLE IF NOT EXISTS routing_dead_letters (
    tenant_id TEXT NOT NULL,
    event_id TEXT NOT NULL,
    destination TEXT NOT NULL,
    route_rules TEXT[] NOT NULL DEFAULT '{}',
    envelope JSONB NOT NULL,
    attempts INTEGER NOT NULL,
    error TEXT NOT NULL,
    failed_at TIMESTAMPTZ NOT NULL,
    failure_count INTEGER NOT NULL DEFAULT 1,
    PRIMARY KEY (tenant_id, event_id, destination)
);

CREATE INDEX IF NOT EXISTS routing_dead_letters_tenant_time_idx
    ON routing_dead_letters (tenant_id, failed_at DESC);

CREATE TABLE IF NOT EXISTS routing_destination_health (
    tenant_id TEXT NOT NULL,
    destination TEXT NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('unknown','healthy','degraded','unhealthy')),
    consecutive_failures INTEGER NOT NULL DEFAULT 0,
    last_success_at TIMESTAMPTZ,
    last_failure_at TIMESTAMPTZ,
    last_error TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, destination)
);
