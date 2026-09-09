package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
)

// Summary is the compact operational snapshot shown at the top of the
// TelemetryForge dashboard.
type Summary struct {
	WindowSeconds int64   `json:"window_seconds"`
	Events        int64   `json:"events"`
	EventsPerSec  float64 `json:"events_per_second"`
	ErrorCount    int64   `json:"error_count"`
	ErrorRate     float64 `json:"error_rate"`
	P95LatencyMS  float64 `json:"p95_latency_ms"`
	ActiveSources int64   `json:"active_sources"`
}

// Incident describes one frozen Flight Recorder window.
type Incident struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Status        string    `json:"status"`
	TriggerReason string    `json:"trigger_reason,omitempty"`
	FrozenFrom    time.Time `json:"frozen_from"`
	FrozenTo      time.Time `json:"frozen_to"`
	DetectedAt    time.Time `json:"detected_at"`
	EventCount    int64     `json:"event_count"`
}

// LiveRecord pairs the canonical envelope with server ingestion time. The
// ingestion timestamp, not the source event timestamp, drives the live cursor.
type LiveRecord struct {
	Event      domain.Event
	IngestedAt time.Time
}

// DashboardReader is the extra read contract used by the v0.6.0 UI.
type DashboardReader interface {
	Reader
	Summary(ctx context.Context, window time.Duration) (Summary, error)
	ListIncidents(ctx context.Context, limit int) ([]Incident, error)
	IncidentEvents(ctx context.Context, incidentID string, limit int) ([]domain.Event, error)
	LiveEvents(ctx context.Context, after time.Time, afterID string, limit int) ([]LiveRecord, error)
}

// Summary aggregates recent stored telemetry.
//
// Error identification deliberately uses a small, documented convention for
// v0.6.0: event types containing "error" or tags with severity/error values.
// Later schema/policy releases will make this configurable per source.
func (store *PostgresStore) Summary(ctx context.Context, window time.Duration) (Summary, error) {
	if window <= 0 {
		window = 5 * time.Minute
	}

	var summary Summary
	summary.WindowSeconds = int64(window.Seconds())

	err := store.pool.QueryRow(ctx, `
		SELECT
			COUNT(*)::bigint,
			COUNT(DISTINCT source)::bigint,
			COUNT(*) FILTER (
				WHERE lower(event_type) LIKE '%error%'
				   OR lower(COALESCE(tags->>'severity','')) IN ('error','critical','fatal')
				   OR lower(COALESCE(tags->>'level','')) IN ('error','critical','fatal')
			)::bigint,
			COALESCE(
				percentile_cont(0.95) WITHIN GROUP (ORDER BY metric_value)
				FILTER (
					WHERE metric_value IS NOT NULL
					  AND lower(COALESCE(metric_unit,'')) = 'ms'
					  AND (
						lower(event_type) LIKE '%latency%'
						OR lower(event_type) LIKE '%duration%'
					  )
				),
				0
			)::double precision
		FROM telemetry_events
		WHERE event_time >= now() - ($1 * interval '1 second')`,
		summary.WindowSeconds,
	).Scan(&summary.Events, &summary.ActiveSources, &summary.ErrorCount, &summary.P95LatencyMS)
	if err != nil {
		return Summary{}, fmt.Errorf("query dashboard summary: %w", err)
	}

	if summary.WindowSeconds > 0 {
		summary.EventsPerSec = float64(summary.Events) / float64(summary.WindowSeconds)
	}
	if summary.Events > 0 {
		summary.ErrorRate = float64(summary.ErrorCount) / float64(summary.Events)
	}

	return summary, nil
}

// ListIncidents returns newest detected incidents with their frozen event count.
func (store *PostgresStore) ListIncidents(ctx context.Context, limit int) ([]Incident, error) {
	if limit < 1 || limit > 100 {
		limit = 25
	}

	rows, err := store.pool.Query(ctx, `
		SELECT i.incident_id,
		       i.title,
		       i.status,
		       COALESCE(i.trigger_reason,''),
		       i.frozen_from,
		       i.frozen_to,
		       i.detected_at,
		       COUNT(DISTINCT e.event_id)::bigint
		  FROM incidents i
		  LEFT JOIN incident_events e ON e.incident_id = i.incident_id
		 GROUP BY i.incident_id, i.title, i.status, i.trigger_reason,
		          i.frozen_from, i.frozen_to, i.detected_at
		 ORDER BY i.detected_at DESC
		 LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("query incidents: %w", err)
	}
	defer rows.Close()

	incidents := make([]Incident, 0, limit)
	for rows.Next() {
		var incident Incident
		if err := rows.Scan(
			&incident.ID,
			&incident.Title,
			&incident.Status,
			&incident.TriggerReason,
			&incident.FrozenFrom,
			&incident.FrozenTo,
			&incident.DetectedAt,
			&incident.EventCount,
		); err != nil {
			return nil, fmt.Errorf("scan incident: %w", err)
		}
		incidents = append(incidents, incident)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate incidents: %w", err)
	}
	return incidents, nil
}

// AnnotateIncident records why an incident was created.
func (store *PostgresStore) AnnotateIncident(ctx context.Context, incidentID, reason string) error {
	_, err := store.pool.Exec(ctx, `
		UPDATE incidents
		   SET trigger_reason = $2,
		       detected_at = now()
		 WHERE incident_id = $1`,
		incidentID, reason)
	if err != nil {
		return fmt.Errorf("annotate incident: %w", err)
	}
	return nil
}

// IncidentEvents reconstructs canonical envelopes captured for one incident.
func (store *PostgresStore) IncidentEvents(ctx context.Context, incidentID string, limit int) ([]domain.Event, error) {
	if limit < 1 || limit > 1000 {
		limit = 250
	}

	rows, err := store.pool.Query(ctx, `
		SELECT envelope
		  FROM (
			SELECT DISTINCT ON (event_id)
			       event_id, event_time, captured_at, envelope
			  FROM incident_events
			 WHERE incident_id = $1
			 ORDER BY event_id, captured_at ASC
		  ) AS captured
		 ORDER BY event_time ASC, event_id ASC
		 LIMIT $2`,
		incidentID, limit)
	if err != nil {
		return nil, fmt.Errorf("query incident events: %w", err)
	}
	defer rows.Close()

	events := make([]domain.Event, 0, limit)
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan incident envelope: %w", err)
		}
		var event domain.Event
		if err := json.Unmarshal(raw, &event); err != nil {
			return nil, fmt.Errorf("decode incident envelope: %w", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate incident events: %w", err)
	}
	return events, nil
}

// LiveEvents returns newly ingested records in stable chronological order.
//
// Source event_time is intentionally not used as the cursor. Telemetry often
// arrives late or out of order, so live delivery must follow server
// `ingested_at` order instead.
func (store *PostgresStore) LiveEvents(ctx context.Context, after time.Time, afterID string, limit int) ([]LiveRecord, error) {
	if limit < 1 || limit > 1000 {
		limit = 500
	}

	rows, err := store.pool.Query(ctx, `
		SELECT event_id, source, event_type, event_time, tags, payload,
		       metric_value, COALESCE(metric_unit, ''), schema_version,
		       COALESCE(correlation_id, ''), ingested_at
		  FROM telemetry_events
		 WHERE (ingested_at, event_id) > ($1, $2)
		 ORDER BY ingested_at ASC, event_id ASC
		 LIMIT $3`,
		after.UTC(), afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("query live telemetry: %w", err)
	}
	defer rows.Close()

	records := make([]LiveRecord, 0, limit)
	for rows.Next() {
		var record LiveRecord
		var tags []byte
		var payload []byte
		if err := rows.Scan(
			&record.Event.ID,
			&record.Event.Source,
			&record.Event.Type,
			&record.Event.Timestamp,
			&tags,
			&payload,
			&record.Event.Value,
			&record.Event.Unit,
			&record.Event.SchemaVersion,
			&record.Event.CorrelationID,
			&record.IngestedAt,
		); err != nil {
			return nil, fmt.Errorf("scan live telemetry: %w", err)
		}
		if len(tags) > 0 {
			if err := json.Unmarshal(tags, &record.Event.Tags); err != nil {
				return nil, fmt.Errorf("decode live event tags: %w", err)
			}
		}
		if len(payload) > 0 {
			record.Event.Payload = append(json.RawMessage(nil), payload...)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate live telemetry: %w", err)
	}
	return records, nil
}
