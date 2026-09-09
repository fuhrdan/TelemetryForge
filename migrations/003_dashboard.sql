-- TelemetryForge v0.6.0 dashboard and automatic-incident metadata.

ALTER TABLE incidents
    ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'open';

ALTER TABLE incidents
    ADD COLUMN IF NOT EXISTS trigger_reason TEXT;

ALTER TABLE incidents
    ADD COLUMN IF NOT EXISTS detected_at TIMESTAMPTZ NOT NULL DEFAULT now();

CREATE INDEX IF NOT EXISTS incidents_detected_at_idx
    ON incidents (detected_at DESC);

CREATE INDEX IF NOT EXISTS incidents_status_detected_idx
    ON incidents (status, detected_at DESC);
