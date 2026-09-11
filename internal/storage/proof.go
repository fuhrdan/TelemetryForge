package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/fuhrdan/TelemetryForge/internal/proof"
	"github.com/jackc/pgx/v5"
	"strings"
	"time"
)

func (s *PostgresStore) RecordOperationalProof(ctx context.Context, run proof.Run, sha string, bytes int64) error {
	if err := run.Validate(); err != nil {
		return err
	}
	if len(sha) != 64 || bytes <= 0 {
		return errors.New("valid artifact sha256/bytes required")
	}
	payload, err := json.Marshal(run)
	if err != nil {
		return err
	}
	var oldSHA string
	var oldBytes int64
	err = s.pool.QueryRow(ctx, `SELECT artifact_sha256,artifact_bytes FROM operational_proofs WHERE run_id=$1`, run.RunID).Scan(&oldSHA, &oldBytes)
	if err == nil {
		if oldSHA == sha && oldBytes == bytes {
			return nil
		}
		return fmt.Errorf("proof run_id %q already recorded with different artifact", run.RunID)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("check proof identity: %w", err)
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO operational_proofs(run_id,scenario,status,started_at,completed_at,artifact_sha256,artifact_bytes,artifact) VALUES($1,$2,$3,$4,$5,$6,$7,$8::jsonb)`, run.RunID, run.Scenario, run.Status, run.StartedAt.UTC(), run.CompletedAt.UTC(), strings.ToLower(sha), bytes, string(payload))
	if err != nil {
		return fmt.Errorf("record operational proof: %w", err)
	}
	return nil
}
func (s *PostgresStore) ListOperationalProofs(ctx context.Context, limit int) ([]proof.Stored, error) {
	if limit < 1 || limit > 200 {
		limit = 25
	}
	rows, err := s.pool.Query(ctx, `SELECT artifact,artifact_sha256,artifact_bytes,recorded_at FROM operational_proofs ORDER BY recorded_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]proof.Stored, 0, limit)
	for rows.Next() {
		var b []byte
		var x proof.Stored
		if err := rows.Scan(&b, &x.ArtifactSHA256, &x.ArtifactBytes, &x.RecordedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(b, &x.Run); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *PostgresStore) PruneOperationalProofs(ctx context.Context, before time.Time) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM operational_proofs WHERE recorded_at < $1`, before.UTC())
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
