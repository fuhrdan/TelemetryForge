package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/incidentarchive"
	"github.com/fuhrdan/TelemetryForge/internal/security"
	"github.com/jackc/pgx/v5"
)

// IncidentArchiveData returns one tenant-scoped incident plus the first
// captured copy of every canonical event. It preserves the Flight Recorder
// captured_at timestamp used by the portable .tfincident format.
func (store *PostgresStore) IncidentArchiveData(
	ctx context.Context,
	incidentID string,
	limit int,
) (incidentarchive.IncidentMetadata, []incidentarchive.EventRecord, error) {
	if strings.TrimSpace(incidentID) == "" {
		return incidentarchive.IncidentMetadata{}, nil, errors.New("incident ID is required")
	}
	if limit < 1 || limit > incidentarchive.MaxEvents {
		limit = 10000
	}

	var incident incidentarchive.IncidentMetadata
	err := store.pool.QueryRow(ctx, `
		SELECT incident_id, title, status, COALESCE(trigger_reason,''),
		       frozen_from, frozen_to, detected_at
		  FROM incidents
		 WHERE tenant_id = $1
		   AND incident_id = $2`,
		security.TenantID(ctx), incidentID,
	).Scan(
		&incident.ID,
		&incident.Title,
		&incident.Status,
		&incident.TriggerReason,
		&incident.FrozenFrom,
		&incident.FrozenTo,
		&incident.DetectedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return incidentarchive.IncidentMetadata{}, nil, fmt.Errorf("incident %q not found", incidentID)
		}
		return incidentarchive.IncidentMetadata{}, nil, fmt.Errorf("query incident metadata: %w", err)
	}

	rows, err := store.pool.Query(ctx, `
		SELECT captured_at, envelope
		  FROM (
			SELECT DISTINCT ON (event_id)
			       event_id, event_time, captured_at, envelope
			  FROM incident_events
			 WHERE tenant_id = $1
			   AND incident_id = $2
			 ORDER BY event_id, captured_at ASC
		  ) AS captured
		 ORDER BY event_time ASC, event_id ASC
		 LIMIT $3`,
		security.TenantID(ctx), incidentID, limit)
	if err != nil {
		return incidentarchive.IncidentMetadata{}, nil, fmt.Errorf("query incident archive events: %w", err)
	}
	defer rows.Close()

	records := make([]incidentarchive.EventRecord, 0, limit)
	for rows.Next() {
		var record incidentarchive.EventRecord
		var envelope []byte
		if err := rows.Scan(&record.CapturedAt, &envelope); err != nil {
			return incidentarchive.IncidentMetadata{}, nil, fmt.Errorf("scan incident archive event: %w", err)
		}
		if err := json.Unmarshal(envelope, &record.Event); err != nil {
			return incidentarchive.IncidentMetadata{}, nil, fmt.Errorf("decode incident archive event: %w", err)
		}
		if record.Event.TenantID == "" {
			record.Event.TenantID = security.TenantID(ctx)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return incidentarchive.IncidentMetadata{}, nil, fmt.Errorf("iterate incident archive events: %w", err)
	}
	return incident, records, nil
}

// ImportIncidentArchive inserts a portable incident into the current trusted
// tenant. Existing incident IDs are never overwritten.
func (store *PostgresStore) ImportIncidentArchive(
	ctx context.Context,
	incident incidentarchive.IncidentMetadata,
	records []incidentarchive.EventRecord,
	provenance incidentarchive.ImportProvenance,
) (int64, error) {
	tenantID := security.TenantID(ctx)
	if strings.TrimSpace(incident.ID) == "" {
		return 0, errors.New("incident ID is required")
	}
	if len(records) == 0 {
		return 0, errors.New("incident archive contains no events")
	}
	if len(records) > incidentarchive.MaxEvents {
		return 0, fmt.Errorf("incident archive contains too many events: %d", len(records))
	}

	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin incident archive import: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	tag, err := tx.Exec(ctx, `
		INSERT INTO incidents
			(tenant_id, incident_id, title, status, trigger_reason,
			 frozen_from, frozen_to, detected_at)
		VALUES ($1,$2,$3,$4,NULLIF($5,''),$6,$7,$8)
		ON CONFLICT (tenant_id, incident_id) DO NOTHING`,
		tenantID, incident.ID, incident.Title, incident.Status,
		incident.TriggerReason, incident.FrozenFrom.UTC(),
		incident.FrozenTo.UTC(), incident.DetectedAt.UTC())
	if err != nil {
		return 0, fmt.Errorf("insert imported incident: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return 0, fmt.Errorf("incident %q already exists in tenant %q", incident.ID, tenantID)
	}

	var inserted int64
	for _, record := range records {
		event := record.Event
		event.TenantID = tenantID
		if err := event.Validate(); err != nil {
			return 0, fmt.Errorf("import event %q is invalid: %w", event.ID, err)
		}
		envelope, err := json.Marshal(event)
		if err != nil {
			return 0, fmt.Errorf("encode imported event %q: %w", event.ID, err)
		}

		capturedAt := record.CapturedAt
		if capturedAt.IsZero() {
			capturedAt = event.Timestamp
		}

		tag, err := tx.Exec(ctx, `
			INSERT INTO incident_events
				(tenant_id, incident_id, event_id, captured_at, event_time,
				 source, event_type, envelope)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb)
			ON CONFLICT DO NOTHING`,
			tenantID, incident.ID, event.ID, capturedAt.UTC(),
			event.Timestamp.UTC(), event.Source, event.Type, string(envelope))
		if err != nil {
			return 0, fmt.Errorf("insert imported incident event %q: %w", event.ID, err)
		}
		inserted += tag.RowsAffected()
	}

	if strings.TrimSpace(provenance.ArchiveID) == "" {
		return 0, errors.New("archive provenance ID is required")
	}
	provenanceTag, err := tx.Exec(ctx, `
		INSERT INTO incident_archive_imports
			(tenant_id, archive_id, source_tenant_id, source_incident_id,
			 imported_incident_id, format_version, telemetryforge_version,
			 file_sha256, encrypted)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (tenant_id, archive_id) DO NOTHING`,
		tenantID, provenance.ArchiveID, provenance.SourceTenantID,
		provenance.SourceIncidentID, incident.ID, provenance.FormatVersion,
		provenance.TelemetryForgeVersion, provenance.FileSHA256,
		provenance.Encrypted)
	if err != nil {
		return 0, fmt.Errorf("record incident archive import provenance: %w", err)
	}
	if provenanceTag.RowsAffected() == 0 {
		return 0, fmt.Errorf("archive %q was already imported into tenant %q",
			provenance.ArchiveID, tenantID)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit incident archive import: %w", err)
	}
	return inserted, nil
}

// NormalizeImportedEvents rewrites embedded tenant IDs for an explicit trusted
// tenant remap. It is separated from SQL so CLI and future importers can make
// the remap decision visible before persistence.
func NormalizeImportedEvents(
	records []incidentarchive.EventRecord,
	tenantID string,
) []incidentarchive.EventRecord {
	result := make([]incidentarchive.EventRecord, len(records))
	for index, record := range records {
		result[index] = record
		result[index].Event = cloneArchiveEvent(record.Event)
		result[index].Event.TenantID = tenantID
	}
	return result
}

func cloneArchiveEvent(event domain.Event) domain.Event {
	result := event
	if event.Tags != nil {
		result.Tags = make(map[string]string, len(event.Tags))
		for key, value := range event.Tags {
			result.Tags[key] = value
		}
	}
	if event.Payload != nil {
		result.Payload = append(json.RawMessage(nil), event.Payload...)
	}
	return result
}

// ListIncidentArchiveImports returns newest-first tenant-scoped import
// provenance without exposing archived telemetry payloads.
func (store *PostgresStore) ListIncidentArchiveImports(
	ctx context.Context,
	limit int,
) ([]incidentarchive.ImportProvenance, error) {
	if limit < 1 || limit > 200 {
		limit = 25
	}

	rows, err := store.pool.Query(ctx, `
		SELECT archive_id, source_tenant_id, source_incident_id,
		       imported_incident_id, format_version, telemetryforge_version,
		       file_sha256, encrypted, imported_at
		  FROM incident_archive_imports
		 WHERE tenant_id = $1
		 ORDER BY imported_at DESC
		 LIMIT $2`,
		security.TenantID(ctx), limit)
	if err != nil {
		return nil, fmt.Errorf("list incident archive imports: %w", err)
	}
	defer rows.Close()

	result := make([]incidentarchive.ImportProvenance, 0, limit)
	for rows.Next() {
		var item incidentarchive.ImportProvenance
		if err := rows.Scan(
			&item.ArchiveID,
			&item.SourceTenantID,
			&item.SourceIncidentID,
			&item.ImportedIncidentID,
			&item.FormatVersion,
			&item.TelemetryForgeVersion,
			&item.FileSHA256,
			&item.Encrypted,
			&item.ImportedAt,
		); err != nil {
			return nil, fmt.Errorf("scan incident archive import: %w", err)
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate incident archive imports: %w", err)
	}
	return result, nil
}
