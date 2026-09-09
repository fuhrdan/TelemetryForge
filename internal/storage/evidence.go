package storage

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/fuhrdan/TelemetryForge/internal/evidence"
	"github.com/fuhrdan/TelemetryForge/internal/security"
	"github.com/jackc/pgx/v5"
)

// SaveEvidenceGraph persists the latest generated graph for one tenant/incident.
//
// The source of truth remains frozen incident evidence; this snapshot is an
// investigation artifact that can be regenerated when graph logic changes.
func (store *PostgresStore) SaveEvidenceGraph(ctx context.Context, graph evidence.Graph) error {
	payload, err := json.Marshal(graph)
	if err != nil {
		return fmt.Errorf("encode evidence graph: %w", err)
	}
	_, err = store.pool.Exec(ctx, `
		INSERT INTO evidence_graph_snapshots
			(tenant_id, incident_id, generated_at, node_count, edge_count, graph)
		VALUES ($1,$2,$3,$4,$5,$6::jsonb)
		ON CONFLICT (tenant_id, incident_id) DO UPDATE SET
			generated_at = EXCLUDED.generated_at,
			node_count = EXCLUDED.node_count,
			edge_count = EXCLUDED.edge_count,
			graph = EXCLUDED.graph`,
		security.TenantID(ctx), graph.IncidentID, graph.GeneratedAt,
		graph.Summary.NodeCount, graph.Summary.EdgeCount, string(payload))
	if err != nil {
		return fmt.Errorf("persist evidence graph: %w", err)
	}
	return nil
}

// EvidenceGraph returns the latest stored snapshot when one already exists.
func (store *PostgresStore) EvidenceGraph(ctx context.Context, incidentID string) (evidence.Graph, bool, error) {
	var raw []byte
	err := store.pool.QueryRow(ctx, `
		SELECT graph
		  FROM evidence_graph_snapshots
		 WHERE tenant_id = $1
		   AND incident_id = $2`,
		security.TenantID(ctx), incidentID,
	).Scan(&raw)
	if err != nil {
		if err == pgx.ErrNoRows {
			return evidence.Graph{}, false, nil
		}
		return evidence.Graph{}, false, fmt.Errorf("query evidence graph: %w", err)
	}

	var graph evidence.Graph
	if err := json.Unmarshal(raw, &graph); err != nil {
		return evidence.Graph{}, false, fmt.Errorf("decode evidence graph: %w", err)
	}
	return graph, true, nil
}
