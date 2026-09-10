package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/fuhrdan/TelemetryForge/internal/connectors"
	"time"
)

func (store *PostgresStore) RecordConnectorRuntimeState(ctx context.Context, state connectors.RuntimeState) error {
	signals, err := json.Marshal(state.Capabilities.Signals)
	if err != nil {
		return fmt.Errorf("encode connector signals: %w", err)
	}
	_, err = store.pool.Exec(ctx, `INSERT INTO connector_runtime_state (instance_id,destination,connector_kind,protocol,signals,health_check,retry_classification,ready,last_error,checked_at) VALUES ($1,$2,$3,$4,$5::jsonb,$6,$7,$8,$9,$10) ON CONFLICT (instance_id,destination) DO UPDATE SET connector_kind=EXCLUDED.connector_kind,protocol=EXCLUDED.protocol,signals=EXCLUDED.signals,health_check=EXCLUDED.health_check,retry_classification=EXCLUDED.retry_classification,ready=EXCLUDED.ready,last_error=EXCLUDED.last_error,checked_at=EXCLUDED.checked_at`, state.InstanceID, state.Destination, state.Kind, state.Capabilities.Protocol, string(signals), state.Capabilities.HealthCheck, state.Capabilities.RetryClassification, state.Ready, truncateConnectorError(state.LastError), state.CheckedAt.UTC())
	if err != nil {
		return fmt.Errorf("record connector runtime state: %w", err)
	}
	return nil
}
func (store *PostgresStore) ListConnectorRuntimeStates(ctx context.Context, limit int) ([]connectors.RuntimeState, error) {
	if limit < 1 || limit > 500 {
		limit = 100
	}
	rows, err := store.pool.Query(ctx, `SELECT instance_id,destination,connector_kind,protocol,signals,health_check,retry_classification,ready,last_error,checked_at FROM connector_runtime_state WHERE checked_at >= now() - interval '2 minutes' ORDER BY destination ASC, checked_at DESC, instance_id ASC LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list connector runtime state: %w", err)
	}
	defer rows.Close()
	result := make([]connectors.RuntimeState, 0, limit)
	for rows.Next() {
		var state connectors.RuntimeState
		var signals []byte
		if err := rows.Scan(&state.InstanceID, &state.Destination, &state.Kind, &state.Capabilities.Protocol, &signals, &state.Capabilities.HealthCheck, &state.Capabilities.RetryClassification, &state.Ready, &state.LastError, &state.CheckedAt); err != nil {
			return nil, fmt.Errorf("scan connector runtime state: %w", err)
		}
		state.Capabilities.Kind = state.Kind
		if err := json.Unmarshal(signals, &state.Capabilities.Signals); err != nil {
			return nil, fmt.Errorf("decode connector runtime signals: %w", err)
		}
		result = append(result, state)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate connector runtime states: %w", err)
	}
	return result, nil
}
func (store *PostgresStore) PruneConnectorRuntimeStates(ctx context.Context, before time.Time) (int64, error) {
	tag, err := store.pool.Exec(ctx, `DELETE FROM connector_runtime_state WHERE checked_at < $1`, before.UTC())
	if err != nil {
		return 0, fmt.Errorf("prune connector runtime state: %w", err)
	}
	return tag.RowsAffected(), nil
}
func truncateConnectorError(value string) string {
	const maximum = 2048
	if len(value) <= maximum {
		return value
	}
	return value[:maximum]
}
