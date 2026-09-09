package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/costsim"
	"github.com/fuhrdan/TelemetryForge/internal/replay"
	"github.com/fuhrdan/TelemetryForge/internal/security"
)

// StartReplay creates one durable replay-history row before processing starts.
func (store *PostgresStore) StartReplay(ctx context.Context, run replay.Run) error {
	_, err := store.pool.Exec(ctx, `
        INSERT INTO replay_runs
            (tenant_id, run_id, incident_id, mode, status, active_policy,
             active_version, shadow_policy, shadow_version, output_topic, started_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7,NULLIF($8,''),NULLIF($9,''),NULLIF($10,''),$11)`,
		security.TenantID(ctx), run.ID, run.IncidentID, run.Mode, run.Status,
		run.ActivePolicy, run.ActiveVersion, run.ShadowPolicy, run.ShadowVersion,
		run.OutputTopic, run.StartedAt)
	if err != nil {
		return fmt.Errorf("insert replay run: %w", err)
	}
	return nil
}

// RecordReplayEvent stores compact per-event replay effects.
func (store *PostgresStore) RecordReplayEvent(ctx context.Context, result replay.EventResult) error {
	_, err := store.pool.Exec(ctx, `
        INSERT INTO replay_event_results
            (tenant_id, run_id, event_id, changed, dropped_tag_count, quarantined,
             finding_count, shadow_diff_count)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
        ON CONFLICT (run_id, event_id) DO UPDATE SET
            changed = EXCLUDED.changed,
            dropped_tag_count = EXCLUDED.dropped_tag_count,
            quarantined = EXCLUDED.quarantined,
            finding_count = EXCLUDED.finding_count,
            shadow_diff_count = EXCLUDED.shadow_diff_count`,
		security.TenantID(ctx), result.RunID, result.EventID, result.Changed,
		result.DroppedTagCount, result.Quarantined, result.FindingCount,
		result.ShadowDiffCount)
	if err != nil {
		return fmt.Errorf("insert replay event result: %w", err)
	}
	return nil
}

// CompleteReplay updates the durable replay summary after success or failure.
func (store *PostgresStore) CompleteReplay(ctx context.Context, run replay.Run) error {
	_, err := store.pool.Exec(ctx, `
        UPDATE replay_runs
           SET status = $3,
               completed_at = $4,
               event_count = $5,
               changed_event_count = $6,
               dropped_tag_count = $7,
               quarantined_count = $8,
               finding_count = $9,
               shadow_diff_count = $10,
               published_count = $11,
               error = NULLIF($12,'')
         WHERE tenant_id = $1
           AND run_id = $2`,
		security.TenantID(ctx), run.ID, run.Status, run.CompletedAt, run.EventCount, run.ChangedEventCount,
		run.DroppedTagCount, run.QuarantinedCount, run.FindingCount,
		run.ShadowDiffCount, run.PublishedCount, run.Error)
	if err != nil {
		return fmt.Errorf("update replay run: %w", err)
	}
	return nil
}

// ListReplayRuns returns newest-first replay summaries for the dashboard.
func (store *PostgresStore) ListReplayRuns(ctx context.Context, limit int) ([]replay.Run, error) {
	if limit < 1 || limit > 200 {
		limit = 25
	}
	rows, err := store.pool.Query(ctx, `
        SELECT run_id, incident_id, mode, status, active_policy, active_version,
               COALESCE(shadow_policy,''), COALESCE(shadow_version,''),
               COALESCE(output_topic,''), started_at,
               COALESCE(completed_at, started_at), (completed_at IS NOT NULL),
               event_count, changed_event_count, dropped_tag_count,
               quarantined_count, finding_count, shadow_diff_count,
               published_count, COALESCE(error,'')
          FROM replay_runs
         WHERE tenant_id = $1
         ORDER BY started_at DESC
         LIMIT $2`, security.TenantID(ctx), limit)
	if err != nil {
		return nil, fmt.Errorf("query replay runs: %w", err)
	}
	defer rows.Close()

	result := make([]replay.Run, 0, limit)
	for rows.Next() {
		var run replay.Run
		var completedAt time.Time
		var hasCompletedAt bool
		if err := rows.Scan(
			&run.ID, &run.IncidentID, &run.Mode, &run.Status, &run.ActivePolicy,
			&run.ActiveVersion, &run.ShadowPolicy, &run.ShadowVersion,
			&run.OutputTopic, &run.StartedAt, &completedAt, &hasCompletedAt,
			&run.EventCount, &run.ChangedEventCount, &run.DroppedTagCount,
			&run.QuarantinedCount, &run.FindingCount, &run.ShadowDiffCount,
			&run.PublishedCount, &run.Error,
		); err != nil {
			return nil, fmt.Errorf("scan replay run: %w", err)
		}
		if hasCompletedAt {
			run.CompletedAt = &completedAt
		}
		result = append(result, run)
	}
	return result, rows.Err()
}

// SaveCostSimulation writes one completed or failed cost projection.
func (store *PostgresStore) SaveCostSimulation(ctx context.Context, result costsim.Result) error {
	assumptions, err := json.Marshal(result.Assumptions)
	if err != nil {
		return fmt.Errorf("encode cost assumptions: %w", err)
	}

	_, err = store.pool.Exec(ctx, `
        INSERT INTO cost_simulations
            (tenant_id, simulation_id, incident_id, active_policy, active_version,
             shadow_policy, shadow_version, pricing_model, currency,
             started_at, completed_at, window_seconds, event_count,
             baseline_bytes, active_bytes, shadow_bytes,
             baseline_series, active_series, shadow_series,
             projected_monthly_baseline_gb, projected_monthly_active_gb,
             projected_monthly_shadow_gb, projected_monthly_baseline_cost,
             projected_monthly_active_cost, projected_monthly_shadow_cost,
             active_changed_events, shadow_changed_events, assumptions, error)
        VALUES
            ($1,$2,$3,$4,$5,NULLIF($6,''),NULLIF($7,''),NULLIF($8,''),NULLIF($9,''),
             $10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,
             $26,$27,$28::jsonb,NULLIF($29,''))
        ON CONFLICT (simulation_id) DO UPDATE SET
            completed_at = EXCLUDED.completed_at,
            error = EXCLUDED.error`,
		security.TenantID(ctx), result.ID, result.IncidentID, result.ActivePolicy,
		result.ActiveVersion,
		result.ShadowPolicy, result.ShadowVersion, result.PricingModel,
		result.Currency, result.StartedAt, result.CompletedAt, result.WindowSeconds,
		result.EventCount, result.Baseline.Bytes, result.Active.Bytes,
		result.Shadow.Bytes, result.Baseline.Series, result.Active.Series,
		result.Shadow.Series, result.MonthlyBaselineGB, result.MonthlyActiveGB,
		result.MonthlyShadowGB, result.MonthlyBaselineCost, result.MonthlyActiveCost,
		result.MonthlyShadowCost, result.Active.ChangedEvents,
		result.Shadow.ChangedEvents, string(assumptions), result.Error)
	if err != nil {
		return fmt.Errorf("insert cost simulation: %w", err)
	}
	return nil
}

// ListCostSimulations returns newest-first cost projections.
func (store *PostgresStore) ListCostSimulations(ctx context.Context, limit int) ([]costsim.Result, error) {
	if limit < 1 || limit > 200 {
		limit = 25
	}
	rows, err := store.pool.Query(ctx, `
        SELECT simulation_id, incident_id, active_policy, active_version,
               COALESCE(shadow_policy,''), COALESCE(shadow_version,''),
               COALESCE(pricing_model,''), COALESCE(currency,''), started_at,
               completed_at, window_seconds, event_count,
               baseline_bytes, active_bytes, shadow_bytes,
               baseline_series, active_series, shadow_series,
               projected_monthly_baseline_gb, projected_monthly_active_gb,
               projected_monthly_shadow_gb, projected_monthly_baseline_cost,
               projected_monthly_active_cost, projected_monthly_shadow_cost,
               active_changed_events, shadow_changed_events, assumptions,
               COALESCE(error,'')
          FROM cost_simulations
         WHERE tenant_id = $1
         ORDER BY started_at DESC
         LIMIT $2`, security.TenantID(ctx), limit)
	if err != nil {
		return nil, fmt.Errorf("query cost simulations: %w", err)
	}
	defer rows.Close()

	result := make([]costsim.Result, 0, limit)
	for rows.Next() {
		var item costsim.Result
		var assumptions []byte
		if err := rows.Scan(
			&item.ID, &item.IncidentID, &item.ActivePolicy, &item.ActiveVersion,
			&item.ShadowPolicy, &item.ShadowVersion, &item.PricingModel,
			&item.Currency, &item.StartedAt, &item.CompletedAt,
			&item.WindowSeconds, &item.EventCount, &item.Baseline.Bytes,
			&item.Active.Bytes, &item.Shadow.Bytes, &item.Baseline.Series,
			&item.Active.Series, &item.Shadow.Series, &item.MonthlyBaselineGB,
			&item.MonthlyActiveGB, &item.MonthlyShadowGB,
			&item.MonthlyBaselineCost, &item.MonthlyActiveCost,
			&item.MonthlyShadowCost, &item.Active.ChangedEvents,
			&item.Shadow.ChangedEvents, &assumptions, &item.Error,
		); err != nil {
			return nil, fmt.Errorf("scan cost simulation: %w", err)
		}
		if err := json.Unmarshal(assumptions, &item.Assumptions); err != nil {
			return nil, fmt.Errorf("decode cost simulation assumptions: %w", err)
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
