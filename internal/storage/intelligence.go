package storage

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/fuhrdan/TelemetryForge/internal/intelligence"
	"github.com/fuhrdan/TelemetryForge/internal/security"
	"github.com/jackc/pgx/v5"
)

// SaveInvestigation persists the latest deterministic investigation snapshot.
func (store *PostgresStore) SaveInvestigation(ctx context.Context, item intelligence.Investigation) error {
	payload, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("encode intelligence snapshot: %w", err)
	}
	_, err = store.pool.Exec(ctx, `
        INSERT INTO intelligence_snapshots
            (tenant_id, incident_id, generated_at, status, investigation)
        VALUES ($1,$2,$3,$4,$5::jsonb)
        ON CONFLICT (tenant_id, incident_id) DO UPDATE SET
            generated_at=EXCLUDED.generated_at,
            status=EXCLUDED.status,
            investigation=EXCLUDED.investigation`,
		security.TenantID(ctx), item.IncidentID, item.GeneratedAt, item.Status, string(payload))
	if err != nil {
		return fmt.Errorf("persist intelligence snapshot: %w", err)
	}
	return nil
}

// Investigation returns the latest stored snapshot for one tenant/incident.
func (store *PostgresStore) Investigation(ctx context.Context, incidentID string) (intelligence.Investigation, bool, error) {
	var raw []byte
	err := store.pool.QueryRow(ctx, `
        SELECT investigation
          FROM intelligence_snapshots
         WHERE tenant_id=$1 AND incident_id=$2`,
		security.TenantID(ctx), incidentID).Scan(&raw)
	if err != nil {
		if err == pgx.ErrNoRows {
			return intelligence.Investigation{}, false, nil
		}
		return intelligence.Investigation{}, false, fmt.Errorf("query intelligence snapshot: %w", err)
	}
	var item intelligence.Investigation
	if err := json.Unmarshal(raw, &item); err != nil {
		return intelligence.Investigation{}, false, fmt.Errorf("decode intelligence snapshot: %w", err)
	}
	return item, true, nil
}

// ListInvestigations returns newest-first tenant-scoped summaries.
func (store *PostgresStore) ListInvestigations(ctx context.Context, limit int) ([]intelligence.Investigation, error) {
	if limit < 1 || limit > 200 {
		limit = 25
	}
	rows, err := store.pool.Query(ctx, `
        SELECT investigation
          FROM intelligence_snapshots
         WHERE tenant_id=$1
         ORDER BY generated_at DESC
         LIMIT $2`, security.TenantID(ctx), limit)
	if err != nil {
		return nil, fmt.Errorf("list intelligence snapshots: %w", err)
	}
	defer rows.Close()
	result := make([]intelligence.Investigation, 0, limit)
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan intelligence snapshot: %w", err)
		}
		var item intelligence.Investigation
		if err := json.Unmarshal(raw, &item); err != nil {
			return nil, fmt.Errorf("decode intelligence snapshot: %w", err)
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
