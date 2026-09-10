package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/changeintel"
	"github.com/fuhrdan/TelemetryForge/internal/security"
	"github.com/jackc/pgx/v5"
)

// RecordChangeMarker stores normalized change evidence exactly once per tenant.
func (store *PostgresStore) RecordChangeMarker(ctx context.Context, marker changeintel.Marker) error {
	if principal, ok := security.PrincipalFrom(ctx); ok {
		trustedTenant := strings.TrimSpace(principal.TenantID)
		if trustedTenant != "" {
			if marker.TenantID != "" && marker.TenantID != trustedTenant {
				return fmt.Errorf("change marker tenant %q does not match trusted tenant %q", marker.TenantID, trustedTenant)
			}
			marker.TenantID = trustedTenant
		}
	}
	if strings.TrimSpace(marker.TenantID) == "" {
		marker.TenantID = security.TenantID(ctx)
	}
	if err := marker.Validate(); err != nil {
		return err
	}
	_, err := store.pool.Exec(ctx, `
        INSERT INTO change_markers
            (tenant_id,change_id,event_id,source,kind,status,environment,version,
             previous_version,git_sha,build_id,actor,rollback_of,summary,changed_at)
        VALUES ($1,$2,$3,$4,$5,$6,NULLIF($7,''),NULLIF($8,''),NULLIF($9,''),
                NULLIF($10,''),NULLIF($11,''),NULLIF($12,''),NULLIF($13,''),
                NULLIF($14,''),$15)
        ON CONFLICT (tenant_id, change_id) DO UPDATE SET
            status=EXCLUDED.status,
            environment=COALESCE(EXCLUDED.environment,change_markers.environment),
            version=COALESCE(EXCLUDED.version,change_markers.version),
            previous_version=COALESCE(EXCLUDED.previous_version,change_markers.previous_version),
            git_sha=COALESCE(EXCLUDED.git_sha,change_markers.git_sha),
            build_id=COALESCE(EXCLUDED.build_id,change_markers.build_id),
            actor=COALESCE(EXCLUDED.actor,change_markers.actor),
            rollback_of=COALESCE(EXCLUDED.rollback_of,change_markers.rollback_of),
            summary=COALESCE(EXCLUDED.summary,change_markers.summary),
            changed_at=EXCLUDED.changed_at`,
		marker.TenantID, marker.ChangeID, marker.EventID, marker.Source,
		marker.Kind, marker.Status, marker.Environment, marker.Version,
		marker.PreviousVersion, marker.GitSHA, marker.BuildID, marker.Actor,
		marker.RollbackOf, marker.Summary, marker.ChangedAt.UTC())
	if err != nil {
		return fmt.Errorf("record change marker: %w", err)
	}
	return nil
}

// ListChangeMarkers returns newest tenant-scoped operational changes.
func (store *PostgresStore) ListChangeMarkers(ctx context.Context, limit int) ([]changeintel.Marker, error) {
	if limit < 1 || limit > 200 {
		limit = 50
	}
	rows, err := store.pool.Query(ctx, `
        SELECT tenant_id,change_id,event_id,source,kind,status,
               COALESCE(environment,''),COALESCE(version,''),COALESCE(previous_version,''),
               COALESCE(git_sha,''),COALESCE(build_id,''),COALESCE(actor,''),
               COALESCE(rollback_of,''),COALESCE(summary,''),changed_at,recorded_at
          FROM change_markers
         WHERE tenant_id=$1
         ORDER BY changed_at DESC
         LIMIT $2`, security.TenantID(ctx), limit)
	if err != nil {
		return nil, fmt.Errorf("list change markers: %w", err)
	}
	defer rows.Close()
	result := make([]changeintel.Marker, 0, limit)
	for rows.Next() {
		var item changeintel.Marker
		if err := rows.Scan(&item.TenantID, &item.ChangeID, &item.EventID, &item.Source,
			&item.Kind, &item.Status, &item.Environment, &item.Version, &item.PreviousVersion,
			&item.GitSHA, &item.BuildID, &item.Actor, &item.RollbackOf, &item.Summary,
			&item.ChangedAt, &item.RecordedAt); err != nil {
			return nil, fmt.Errorf("scan change marker: %w", err)
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate change markers: %w", err)
	}
	return result, nil
}

// ChangeMarker loads one tenant-scoped change by ID.
func (store *PostgresStore) ChangeMarker(ctx context.Context, changeID string) (changeintel.Marker, error) {
	var item changeintel.Marker
	err := store.pool.QueryRow(ctx, `
        SELECT tenant_id,change_id,event_id,source,kind,status,
               COALESCE(environment,''),COALESCE(version,''),COALESCE(previous_version,''),
               COALESCE(git_sha,''),COALESCE(build_id,''),COALESCE(actor,''),
               COALESCE(rollback_of,''),COALESCE(summary,''),changed_at,recorded_at
          FROM change_markers
         WHERE tenant_id=$1 AND change_id=$2`, security.TenantID(ctx), strings.TrimSpace(changeID)).Scan(
		&item.TenantID, &item.ChangeID, &item.EventID, &item.Source, &item.Kind, &item.Status,
		&item.Environment, &item.Version, &item.PreviousVersion, &item.GitSHA, &item.BuildID,
		&item.Actor, &item.RollbackOf, &item.Summary, &item.ChangedAt, &item.RecordedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return changeintel.Marker{}, fmt.Errorf("change %q not found", changeID)
		}
		return changeintel.Marker{}, fmt.Errorf("query change marker: %w", err)
	}
	return item, nil
}

// ChangesBetween returns bounded change markers overlapping an incident window.
func (store *PostgresStore) ChangesBetween(ctx context.Context, from, to time.Time, limit int) ([]changeintel.Marker, error) {
	if limit < 1 || limit > 200 {
		limit = 100
	}
	rows, err := store.pool.Query(ctx, `
        SELECT tenant_id,change_id,event_id,source,kind,status,
               COALESCE(environment,''),COALESCE(version,''),COALESCE(previous_version,''),
               COALESCE(git_sha,''),COALESCE(build_id,''),COALESCE(actor,''),
               COALESCE(rollback_of,''),COALESCE(summary,''),changed_at,recorded_at
          FROM change_markers
         WHERE tenant_id=$1 AND changed_at >= $2 AND changed_at <= $3
         ORDER BY changed_at ASC
         LIMIT $4`, security.TenantID(ctx), from.UTC(), to.UTC(), limit)
	if err != nil {
		return nil, fmt.Errorf("query change window: %w", err)
	}
	defer rows.Close()
	result := make([]changeintel.Marker, 0, limit)
	for rows.Next() {
		var item changeintel.Marker
		if err := rows.Scan(&item.TenantID, &item.ChangeID, &item.EventID, &item.Source,
			&item.Kind, &item.Status, &item.Environment, &item.Version, &item.PreviousVersion,
			&item.GitSHA, &item.BuildID, &item.Actor, &item.RollbackOf, &item.Summary,
			&item.ChangedAt, &item.RecordedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

// AnalyzeChange compares bounded source aggregates before/after one marker and
// persists the latest regenerable analysis snapshot.
func (store *PostgresStore) AnalyzeChange(ctx context.Context, changeID string, before, after time.Duration) (changeintel.Analysis, error) {
	if before <= 0 {
		before = 15 * time.Minute
	}
	if after <= 0 {
		after = 15 * time.Minute
	}
	if before > 2*time.Hour || after > 2*time.Hour {
		return changeintel.Analysis{}, errors.New("change analysis windows cannot exceed two hours")
	}

	marker, err := store.ChangeMarker(ctx, changeID)
	if err != nil {
		return changeintel.Analysis{}, err
	}
	impacts, err := store.changeSourceImpacts(ctx, marker.ChangedAt, before, after)
	if err != nil {
		return changeintel.Analysis{}, err
	}

	analysis := changeintel.Analysis{
		TenantID: marker.TenantID, Change: marker, GeneratedAt: time.Now().UTC(),
		BeforeWindowSeconds: int64(before.Seconds()), AfterWindowSeconds: int64(after.Seconds()),
		SourceImpacts: impacts,
		Notes: []string{
			"Blast radius is the percentage of assessable observed sources showing a material regression; it is not a service dependency graph.",
			"Sources with fewer than five events on either side are classified as insufficient evidence.",
		},
	}
	analysis.Finalize()
	analysis.Recovery, _ = store.changeRecoveryEvidence(ctx, marker, before)

	payload, err := json.Marshal(analysis)
	if err != nil {
		return changeintel.Analysis{}, err
	}
	_, err = store.pool.Exec(ctx, `
        INSERT INTO change_analysis_snapshots
            (tenant_id,change_id,generated_at,assessment,regressed_sources,
             assessable_sources,blast_radius_percent,analysis)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb)
        ON CONFLICT (tenant_id,change_id) DO UPDATE SET
            generated_at=EXCLUDED.generated_at,assessment=EXCLUDED.assessment,
            regressed_sources=EXCLUDED.regressed_sources,
            assessable_sources=EXCLUDED.assessable_sources,
            blast_radius_percent=EXCLUDED.blast_radius_percent,
            analysis=EXCLUDED.analysis`, security.TenantID(ctx), marker.ChangeID,
		analysis.GeneratedAt, analysis.Assessment, analysis.RegressedSources,
		analysis.AssessableSources, analysis.BlastRadiusPercent, string(payload))
	if err != nil {
		return changeintel.Analysis{}, fmt.Errorf("persist change analysis: %w", err)
	}
	return analysis, nil
}

// ChangeAnalysis returns the latest snapshot when one exists.
func (store *PostgresStore) ChangeAnalysis(ctx context.Context, changeID string) (changeintel.Analysis, bool, error) {
	var payload []byte
	err := store.pool.QueryRow(ctx, `SELECT analysis FROM change_analysis_snapshots WHERE tenant_id=$1 AND change_id=$2`, security.TenantID(ctx), changeID).Scan(&payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return changeintel.Analysis{}, false, nil
	}
	if err != nil {
		return changeintel.Analysis{}, false, err
	}
	var result changeintel.Analysis
	if err := json.Unmarshal(payload, &result); err != nil {
		return changeintel.Analysis{}, false, fmt.Errorf("decode change analysis: %w", err)
	}
	return result, true, nil
}

func (store *PostgresStore) changeSourceImpacts(ctx context.Context, center time.Time, before, after time.Duration) ([]changeintel.SourceImpact, error) {
	rows, err := store.pool.Query(ctx, `
        WITH scoped AS (
          SELECT source,
                 CASE WHEN event_time < $2 THEN 'before' ELSE 'after' END AS phase,
                 event_type,tags,metric_value,metric_unit
            FROM telemetry_events
           WHERE tenant_id=$1 AND event_time >= $3 AND event_time < $4
        )
        SELECT source,phase,COUNT(*)::bigint,
               COUNT(*) FILTER (WHERE lower(event_type) LIKE '%error%' OR lower(COALESCE(tags->>'severity','')) IN ('error','critical','fatal') OR lower(COALESCE(tags->>'level','')) IN ('error','critical','fatal'))::bigint,
               COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY metric_value)
                 FILTER (WHERE metric_value IS NOT NULL AND lower(COALESCE(metric_unit,''))='ms' AND (lower(event_type) LIKE '%latency%' OR lower(event_type) LIKE '%duration%')),0)::double precision
          FROM scoped
         GROUP BY source,phase
         ORDER BY source,phase`, security.TenantID(ctx), center.UTC(), center.Add(-before).UTC(), center.Add(after).UTC())
	if err != nil {
		return nil, fmt.Errorf("query change before/after telemetry: %w", err)
	}
	defer rows.Close()
	type pair struct{ before, after changeintel.WindowStats }
	grouped := map[string]pair{}
	for rows.Next() {
		var source, phase string
		var stats changeintel.WindowStats
		if err := rows.Scan(&source, &phase, &stats.Events, &stats.Errors, &stats.P95LatencyMS); err != nil {
			return nil, err
		}
		if stats.Events > 0 {
			stats.ErrorRate = float64(stats.Errors) / float64(stats.Events)
		}
		current := grouped[source]
		if phase == "before" {
			current.before = stats
		} else {
			current.after = stats
		}
		grouped[source] = current
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	impacts := make([]changeintel.SourceImpact, 0, len(grouped))
	for source, pair := range grouped {
		impacts = append(impacts, changeintel.AssessImpact(source, pair.before, pair.after))
	}
	return impacts, nil
}

func (store *PostgresStore) changeRecoveryEvidence(ctx context.Context, marker changeintel.Marker, baseline time.Duration) (changeintel.RecoveryEvidence, error) {
	var rollback changeintel.Marker
	err := store.pool.QueryRow(ctx, `
        SELECT tenant_id,change_id,event_id,source,kind,status,
               COALESCE(environment,''),COALESCE(version,''),COALESCE(previous_version,''),
               COALESCE(git_sha,''),COALESCE(build_id,''),COALESCE(actor,''),
               COALESCE(rollback_of,''),COALESCE(summary,''),changed_at,recorded_at
          FROM change_markers
         WHERE tenant_id=$1 AND changed_at > $2 AND changed_at <= $3
           AND kind='rollback' AND (rollback_of=$4 OR source=$5)
         ORDER BY CASE WHEN rollback_of=$4 THEN 0 ELSE 1 END, changed_at ASC
         LIMIT 1`, security.TenantID(ctx), marker.ChangedAt, marker.ChangedAt.Add(2*time.Hour), marker.ChangeID, marker.Source).Scan(
		&rollback.TenantID, &rollback.ChangeID, &rollback.EventID, &rollback.Source, &rollback.Kind, &rollback.Status,
		&rollback.Environment, &rollback.Version, &rollback.PreviousVersion, &rollback.GitSHA, &rollback.BuildID,
		&rollback.Actor, &rollback.RollbackOf, &rollback.Summary, &rollback.ChangedAt, &rollback.RecordedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return changeintel.RecoveryEvidence{}, nil
	}
	if err != nil {
		return changeintel.RecoveryEvidence{}, err
	}

	impacts, err := store.changeSourceImpacts(ctx, rollback.ChangedAt, baseline, 10*time.Minute)
	if err != nil {
		return changeintel.RecoveryEvidence{}, err
	}
	recovery := changeintel.RecoveryEvidence{Rollback: &rollback, Reason: "A later rollback marker was observed. Recovery timing is associative evidence, not proof that rollback caused recovery."}
	for _, impact := range impacts {
		if impact.Source == marker.Source {
			recovery.AfterRollback = impact.After
			recovery.Recovered = impact.Assessment == changeintel.AssessmentImproved || (impact.After.ErrorRate < .05 && (impact.After.P95LatencyMS == 0 || impact.After.P95LatencyMS < 500))
			break
		}
	}
	return recovery, nil
}
