package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/security"
)

// WriteFlightEvent appends one full canonical envelope to the short-lived
// flight-recorder buffer.
//
// This intentionally occurs before normalization/persistence in the worker
// pipeline so an incident snapshot retains what the processor actually saw.
func (store *PostgresStore) WriteFlightEvent(ctx context.Context, event domain.Event) error {
	envelope, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode flight-recorder event: %w", err)
	}

	_, err = store.pool.Exec(ctx, `
		INSERT INTO flight_recorder_events
			(tenant_id, event_id, captured_at, event_time, source, event_type, envelope)
		VALUES ($1, $2, now(), $3, $4, $5, $6::jsonb)`,
		tenantForEvent(ctx, event), event.ID, event.Timestamp.UTC(),
		event.Source, event.Type, string(envelope))
	if err != nil {
		return fmt.Errorf("write flight-recorder event: %w", err)
	}
	return nil
}

// FreezeIncident copies a flight-recorder capture window into durable incident
// storage before the rolling retention policy removes it.
func (store *PostgresStore) FreezeIncident(ctx context.Context, incidentID, title string, from, to time.Time) (int64, error) {
	incidentID = strings.TrimSpace(incidentID)
	title = strings.TrimSpace(title)
	if incidentID == "" {
		return 0, errors.New("incident ID is required")
	}
	if title == "" {
		return 0, errors.New("incident title is required")
	}
	if from.IsZero() || to.IsZero() || from.After(to) {
		return 0, errors.New("valid incident time window is required")
	}

	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin incident freeze: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	tenantID := security.TenantID(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO incidents (tenant_id, incident_id, title, frozen_from, frozen_to)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (tenant_id, incident_id)
		DO UPDATE SET title = EXCLUDED.title,
		              frozen_from = EXCLUDED.frozen_from,
		              frozen_to = EXCLUDED.frozen_to`,
		tenantID, incidentID, title, from.UTC(), to.UTC())
	if err != nil {
		return 0, fmt.Errorf("create incident: %w", err)
	}

	result, err := tx.Exec(ctx, `
		INSERT INTO incident_events
			(tenant_id, incident_id, event_id, captured_at, event_time, source, event_type, envelope)
		SELECT $1,
		       $2,
		       captured.event_id,
		       captured.captured_at,
		       captured.event_time,
		       captured.source,
		       captured.event_type,
		       captured.envelope
		  FROM (
			SELECT DISTINCT ON (event_id)
			       event_id, captured_at, event_time, source, event_type, envelope
			  FROM flight_recorder_events
			 WHERE tenant_id = $1
			   AND captured_at >= $3
			   AND captured_at <= $4
			 ORDER BY event_id, captured_at ASC
		  ) AS captured
		ON CONFLICT DO NOTHING`,
		tenantID, incidentID, from.UTC(), to.UTC())
	if err != nil {
		return 0, fmt.Errorf("freeze incident events: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit incident freeze: %w", err)
	}
	return result.RowsAffected(), nil
}
