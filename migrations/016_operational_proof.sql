CREATE TABLE IF NOT EXISTS operational_proofs (
 run_id TEXT PRIMARY KEY, scenario TEXT NOT NULL, status TEXT NOT NULL,
 started_at TIMESTAMPTZ NOT NULL, completed_at TIMESTAMPTZ NOT NULL,
 artifact_sha256 TEXT NOT NULL, artifact_bytes BIGINT NOT NULL,
 artifact JSONB NOT NULL, recorded_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS operational_proofs_recorded_idx ON operational_proofs(recorded_at DESC);
