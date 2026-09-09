-- TelemetryForge v1.0.0 tenant isolation and Evidence Graph.
--
-- Existing pre-v1 data belongs to the explicit "default" tenant. New gateway
-- requests receive tenant_id only from authenticated principal context.

ALTER TABLE event_dedup
    ADD COLUMN IF NOT EXISTS tenant_id TEXT NOT NULL DEFAULT 'default';

ALTER TABLE telemetry_events
    ADD COLUMN IF NOT EXISTS tenant_id TEXT NOT NULL DEFAULT 'default';

ALTER TABLE flight_recorder_events
    ADD COLUMN IF NOT EXISTS tenant_id TEXT NOT NULL DEFAULT 'default';

ALTER TABLE incidents
    ADD COLUMN IF NOT EXISTS tenant_id TEXT NOT NULL DEFAULT 'default';

ALTER TABLE incident_events
    ADD COLUMN IF NOT EXISTS tenant_id TEXT NOT NULL DEFAULT 'default';

ALTER TABLE cardinality_findings
    ADD COLUMN IF NOT EXISTS tenant_id TEXT NOT NULL DEFAULT 'default';

ALTER TABLE policy_shadow_diffs
    ADD COLUMN IF NOT EXISTS tenant_id TEXT NOT NULL DEFAULT 'default';

ALTER TABLE quarantined_events
    ADD COLUMN IF NOT EXISTS tenant_id TEXT NOT NULL DEFAULT 'default';

ALTER TABLE replay_runs
    ADD COLUMN IF NOT EXISTS tenant_id TEXT NOT NULL DEFAULT 'default';

ALTER TABLE replay_event_results
    ADD COLUMN IF NOT EXISTS tenant_id TEXT NOT NULL DEFAULT 'default';

ALTER TABLE cost_simulations
    ADD COLUMN IF NOT EXISTS tenant_id TEXT NOT NULL DEFAULT 'default';

-- Event IDs are idempotent within a tenant. This allows two tenants to use the
-- same client-generated ID without one suppressing the other's telemetry.
DO $$
DECLARE
    definition TEXT;
BEGIN
    SELECT pg_get_constraintdef(oid)
      INTO definition
      FROM pg_constraint
     WHERE conrelid = 'event_dedup'::regclass
       AND contype = 'p';

    IF definition IS NOT NULL
       AND definition <> 'PRIMARY KEY (tenant_id, event_id)' THEN
        ALTER TABLE event_dedup DROP CONSTRAINT event_dedup_pkey;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conrelid = 'event_dedup'::regclass
           AND contype = 'p'
    ) THEN
        ALTER TABLE event_dedup
            ADD PRIMARY KEY (tenant_id, event_id);
    END IF;
END $$;

-- Quarantine is also idempotent inside a tenant.
DO $$
DECLARE
    definition TEXT;
BEGIN
    SELECT pg_get_constraintdef(oid)
      INTO definition
      FROM pg_constraint
     WHERE conrelid = 'quarantined_events'::regclass
       AND contype = 'p';

    IF definition IS NOT NULL
       AND definition <> 'PRIMARY KEY (tenant_id, event_id)' THEN
        ALTER TABLE quarantined_events DROP CONSTRAINT quarantined_events_pkey;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conrelid = 'quarantined_events'::regclass
           AND contype = 'p'
    ) THEN
        ALTER TABLE quarantined_events
            ADD PRIMARY KEY (tenant_id, event_id);
    END IF;
END $$;

-- Incidents are identified inside a tenant. Upgrade the old globally unique
-- incident key and its child foreign key without losing frozen evidence.
ALTER TABLE incident_events
    DROP CONSTRAINT IF EXISTS incident_events_incident_id_fkey;

DO $$
DECLARE
    definition TEXT;
BEGIN
    SELECT pg_get_constraintdef(oid)
      INTO definition
      FROM pg_constraint
     WHERE conrelid = 'incidents'::regclass
       AND contype = 'p';

    IF definition IS NOT NULL
       AND definition <> 'PRIMARY KEY (tenant_id, incident_id)' THEN
        ALTER TABLE incidents DROP CONSTRAINT incidents_pkey;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conrelid = 'incidents'::regclass
           AND contype = 'p'
    ) THEN
        ALTER TABLE incidents
            ADD PRIMARY KEY (tenant_id, incident_id);
    END IF;
END $$;

DO $$
DECLARE
    definition TEXT;
BEGIN
    SELECT pg_get_constraintdef(oid)
      INTO definition
      FROM pg_constraint
     WHERE conrelid = 'incident_events'::regclass
       AND contype = 'p';

    IF definition IS NOT NULL
       AND definition <> 'PRIMARY KEY (tenant_id, incident_id, event_id, captured_at)' THEN
        ALTER TABLE incident_events DROP CONSTRAINT incident_events_pkey;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conrelid = 'incident_events'::regclass
           AND contype = 'p'
    ) THEN
        ALTER TABLE incident_events
            ADD PRIMARY KEY (tenant_id, incident_id, event_id, captured_at);
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'incident_events'::regclass
           AND conname = 'incident_events_tenant_incident_fkey'
    ) THEN
        ALTER TABLE incident_events
            ADD CONSTRAINT incident_events_tenant_incident_fkey
            FOREIGN KEY (tenant_id, incident_id)
            REFERENCES incidents (tenant_id, incident_id)
            ON DELETE CASCADE;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS telemetry_events_tenant_time_idx
    ON telemetry_events (tenant_id, event_time DESC);

CREATE INDEX IF NOT EXISTS telemetry_events_tenant_ingested_idx
    ON telemetry_events (tenant_id, ingested_at, event_id);

CREATE INDEX IF NOT EXISTS flight_recorder_tenant_capture_idx
    ON flight_recorder_events (tenant_id, captured_at DESC);

CREATE INDEX IF NOT EXISTS incidents_tenant_detected_idx
    ON incidents (tenant_id, detected_at DESC);

CREATE INDEX IF NOT EXISTS incident_events_tenant_incident_time_idx
    ON incident_events (tenant_id, incident_id, event_time);

CREATE INDEX IF NOT EXISTS cardinality_findings_tenant_observed_idx
    ON cardinality_findings (tenant_id, observed_at DESC);

CREATE INDEX IF NOT EXISTS policy_shadow_diffs_tenant_observed_idx
    ON policy_shadow_diffs (tenant_id, observed_at DESC);

CREATE INDEX IF NOT EXISTS replay_runs_tenant_started_idx
    ON replay_runs (tenant_id, started_at DESC);

CREATE INDEX IF NOT EXISTS replay_event_results_tenant_run_idx
    ON replay_event_results (tenant_id, run_id);

CREATE INDEX IF NOT EXISTS cost_simulations_tenant_started_idx
    ON cost_simulations (tenant_id, started_at DESC);

CREATE TABLE IF NOT EXISTS evidence_graph_snapshots (
    tenant_id TEXT NOT NULL,
    incident_id TEXT NOT NULL,
    generated_at TIMESTAMPTZ NOT NULL,
    node_count INTEGER NOT NULL,
    edge_count INTEGER NOT NULL,
    graph JSONB NOT NULL,
    PRIMARY KEY (tenant_id, incident_id)
);

CREATE INDEX IF NOT EXISTS evidence_graph_snapshots_generated_idx
    ON evidence_graph_snapshots (tenant_id, generated_at DESC);
