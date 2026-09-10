package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/security"
	"github.com/fuhrdan/TelemetryForge/internal/shaping"
	"github.com/jackc/pgx/v5"
)

// ResolveShapingDecision makes one active shaping decision durable and stable
// across whole-chain retries.
//
// The first decision for tenant/event/config/version wins. If the worker sees
// the same event again because a later processor failed, Resolve returns the
// stored decision instead of incrementing minute statistics a second time or
// letting queue pressure change whether the event is kept.
func (store *PostgresStore) ResolveShapingDecision(
	ctx context.Context,
	decision shaping.Decision,
) (shaping.Decision, bool, error) {
	tenantID := decision.TenantID
	if tenantID == "" {
		tenantID = security.TenantID(ctx)
	}
	decision.TenantID = tenantID

	renamedTags, err := json.Marshal(decision.RenamedTags)
	if err != nil {
		return shaping.Decision{}, false, fmt.Errorf("encode shaping rename metadata: %w", err)
	}

	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return shaping.Decision{}, false, fmt.Errorf("begin shaping decision transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	tag, err := tx.Exec(ctx, `
        INSERT INTO shaping_decisions
            (tenant_id,event_id,config_name,config_version,source,event_type,
             observed_at,rules,keep,protected,protection_reason,base_rate,
             effective_rate,queue_pressure,dropped_tags,renamed_tags,
             payload_dropped,original_bytes,shaped_bytes,reason)
        VALUES
            ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16::jsonb,
             $17,$18,$19,$20)
        ON CONFLICT (tenant_id,event_id,config_name,config_version) DO NOTHING`,
		decision.TenantID, decision.EventID, decision.ConfigName, decision.ConfigVersion,
		decision.Source, decision.EventType, decision.ObservedAt, decision.Rules,
		decision.Keep, decision.Protected, decision.ProtectionReason, decision.BaseRate,
		decision.EffectiveRate, decision.QueuePressure, decision.DroppedTags,
		string(renamedTags), decision.PayloadDropped, decision.OriginalBytes,
		decision.ShapedBytes, decision.Reason)
	if err != nil {
		return shaping.Decision{}, false, fmt.Errorf("insert shaping decision: %w", err)
	}

	if tag.RowsAffected() == 1 {
		if err := recordShapingMinuteStat(ctx, tx, decision); err != nil {
			return shaping.Decision{}, false, err
		}
		if err := tx.Commit(ctx); err != nil {
			return shaping.Decision{}, false, fmt.Errorf("commit shaping decision: %w", err)
		}
		return decision, false, nil
	}

	var existing shaping.Decision
	var existingRenamedTags []byte
	err = tx.QueryRow(ctx, `
        SELECT tenant_id,event_id,source,event_type,observed_at,config_name,
               config_version,rules,keep,protected,protection_reason,base_rate,
               effective_rate,queue_pressure,dropped_tags,renamed_tags,
               payload_dropped,original_bytes,shaped_bytes,reason
          FROM shaping_decisions
         WHERE tenant_id=$1
           AND event_id=$2
           AND config_name=$3
           AND config_version=$4`,
		decision.TenantID, decision.EventID, decision.ConfigName, decision.ConfigVersion,
	).Scan(
		&existing.TenantID, &existing.EventID, &existing.Source, &existing.EventType,
		&existing.ObservedAt, &existing.ConfigName, &existing.ConfigVersion,
		&existing.Rules, &existing.Keep, &existing.Protected,
		&existing.ProtectionReason, &existing.BaseRate, &existing.EffectiveRate,
		&existing.QueuePressure, &existing.DroppedTags, &existingRenamedTags,
		&existing.PayloadDropped, &existing.OriginalBytes, &existing.ShapedBytes,
		&existing.Reason,
	)
	if err != nil {
		return shaping.Decision{}, false, fmt.Errorf("load existing shaping decision: %w", err)
	}
	if len(existingRenamedTags) > 0 {
		if err := json.Unmarshal(existingRenamedTags, &existing.RenamedTags); err != nil {
			return shaping.Decision{}, false, fmt.Errorf("decode shaping rename metadata: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return shaping.Decision{}, false, fmt.Errorf("commit shaping decision lookup: %w", err)
	}
	return existing, true, nil
}

// RecordShapingDecision is retained for operational/tests callers that only
// need to ensure a decision exists. Production workers use ResolveShapingDecision
// so a previously recorded pressure decision can be reused exactly.
func (store *PostgresStore) RecordShapingDecision(ctx context.Context, decision shaping.Decision) error {
	_, _, err := store.ResolveShapingDecision(ctx, decision)
	return err
}

func recordShapingMinuteStat(ctx context.Context, tx pgx.Tx, decision shaping.Decision) error {
	ruleName := "default"
	if len(decision.Rules) > 0 {
		ruleName = strings.Join(decision.Rules, ",")
	}
	transformed := len(decision.DroppedTags) > 0 || len(decision.RenamedTags) > 0 || decision.PayloadDropped
	kept, sampledOut, protected, transformedCount, payloadDropped := int64(0), int64(0), int64(0), int64(0), int64(0)
	if decision.Keep {
		kept = 1
	} else {
		sampledOut = 1
	}
	if decision.Protected {
		protected = 1
	}
	if transformed {
		transformedCount = 1
	}
	if decision.PayloadDropped {
		payloadDropped = 1
	}
	_, err := tx.Exec(ctx, `
        INSERT INTO shaping_minute_stats
            (tenant_id,bucket,config_name,config_version,rule_name,source,event_type,
             observed,kept,sampled_out,protected,transformed,payload_dropped,
             original_bytes,shaped_bytes,updated_at)
        VALUES ($1,date_trunc('minute',$2::timestamptz),$3,$4,$5,$6,$7,
                1,$8,$9,$10,$11,$12,$13,$14,now())
        ON CONFLICT (tenant_id,bucket,config_name,config_version,rule_name,source,event_type)
        DO UPDATE SET
            observed=shaping_minute_stats.observed+1,
            kept=shaping_minute_stats.kept+EXCLUDED.kept,
            sampled_out=shaping_minute_stats.sampled_out+EXCLUDED.sampled_out,
            protected=shaping_minute_stats.protected+EXCLUDED.protected,
            transformed=shaping_minute_stats.transformed+EXCLUDED.transformed,
            payload_dropped=shaping_minute_stats.payload_dropped+EXCLUDED.payload_dropped,
            original_bytes=shaping_minute_stats.original_bytes+EXCLUDED.original_bytes,
            shaped_bytes=shaping_minute_stats.shaped_bytes+EXCLUDED.shaped_bytes,
            updated_at=now()`,
		decision.TenantID, decision.ObservedAt, decision.ConfigName, decision.ConfigVersion,
		ruleName, decision.Source, decision.EventType, kept, sampledOut, protected,
		transformedCount, payloadDropped, decision.OriginalBytes, decision.ShapedBytes)
	if err != nil {
		return fmt.Errorf("record shaping minute statistic: %w", err)
	}
	return nil
}

func (store *PostgresStore) RecordShapingShadowDiff(ctx context.Context, diff shaping.ShadowDiff) error {
	tenantID := diff.TenantID
	if tenantID == "" {
		tenantID = security.TenantID(ctx)
	}
	_, err := store.pool.Exec(ctx, `
        INSERT INTO shaping_shadow_diffs
            (tenant_id,event_id,observed_at,active_config,active_version,
             shadow_config,shadow_version,active_keep,shadow_keep,active_rate,
             shadow_rate,active_effects,shadow_effects)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
        ON CONFLICT
            (tenant_id,event_id,active_config,active_version,shadow_config,shadow_version)
        DO UPDATE SET
            observed_at=EXCLUDED.observed_at,active_keep=EXCLUDED.active_keep,
            shadow_keep=EXCLUDED.shadow_keep,active_rate=EXCLUDED.active_rate,
            shadow_rate=EXCLUDED.shadow_rate,active_effects=EXCLUDED.active_effects,
            shadow_effects=EXCLUDED.shadow_effects`,
		tenantID, diff.EventID, diff.ObservedAt, diff.ActiveConfig, diff.ActiveVersion,
		diff.ShadowConfig, diff.ShadowVersion, diff.ActiveKeep, diff.ShadowKeep,
		diff.ActiveRate, diff.ShadowRate, diff.ActiveEffects, diff.ShadowEffects)
	if err != nil {
		return fmt.Errorf("record shaping shadow diff: %w", err)
	}
	return nil
}

// ShapingSummary aggregates every matching minute row in the window, independent
// of the paginated detail-row limit used by ListShapingStats.
func (store *PostgresStore) ShapingSummary(ctx context.Context, from time.Time) (shaping.Summary, error) {
	if from.IsZero() {
		from = time.Now().UTC().Add(-time.Hour)
	}
	var summary shaping.Summary
	err := store.pool.QueryRow(ctx, `
        SELECT COALESCE(SUM(observed),0),COALESCE(SUM(kept),0),
               COALESCE(SUM(sampled_out),0),COALESCE(SUM(protected),0),
               COALESCE(SUM(transformed),0),COALESCE(SUM(payload_dropped),0),
               COALESCE(SUM(original_bytes),0),COALESCE(SUM(shaped_bytes),0)
          FROM shaping_minute_stats
         WHERE tenant_id=$1 AND bucket >= $2`,
		security.TenantID(ctx), from.UTC(),
	).Scan(
		&summary.Observed, &summary.Kept, &summary.SampledOut, &summary.Protected,
		&summary.Transformed, &summary.PayloadDropped, &summary.OriginalBytes,
		&summary.ShapedBytes,
	)
	if err != nil {
		return shaping.Summary{}, fmt.Errorf("query shaping summary: %w", err)
	}
	return summary, nil
}

func (store *PostgresStore) ListShapingStats(ctx context.Context, from time.Time, limit int) ([]shaping.Stat, error) {
	if limit < 1 || limit > 500 {
		limit = 100
	}
	if from.IsZero() {
		from = time.Now().UTC().Add(-time.Hour)
	}
	rows, err := store.pool.Query(ctx, `
        SELECT bucket,tenant_id,config_name,config_version,rule_name,source,event_type,
               observed,kept,sampled_out,protected,transformed,payload_dropped,
               original_bytes,shaped_bytes
          FROM shaping_minute_stats
         WHERE tenant_id=$1 AND bucket >= $2
         ORDER BY bucket DESC,source,event_type,rule_name
         LIMIT $3`, security.TenantID(ctx), from.UTC(), limit)
	if err != nil {
		return nil, fmt.Errorf("query shaping stats: %w", err)
	}
	defer rows.Close()
	result := make([]shaping.Stat, 0)
	for rows.Next() {
		var item shaping.Stat
		if err := rows.Scan(&item.Bucket, &item.TenantID, &item.ConfigName, &item.ConfigVersion, &item.RuleName, &item.Source, &item.EventType, &item.Observed, &item.Kept, &item.SampledOut, &item.Protected, &item.Transformed, &item.PayloadDropped, &item.OriginalBytes, &item.ShapedBytes); err != nil {
			return nil, fmt.Errorf("scan shaping stats: %w", err)
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate shaping stats: %w", err)
	}
	return result, nil
}

func (store *PostgresStore) ListShapingShadowDiffs(ctx context.Context, limit int) ([]shaping.ShadowDiff, error) {
	if limit < 1 || limit > 500 {
		limit = 100
	}
	rows, err := store.pool.Query(ctx, `
        SELECT tenant_id,event_id,observed_at,active_config,active_version,
               shadow_config,shadow_version,active_keep,shadow_keep,active_rate,
               shadow_rate,active_effects,shadow_effects
          FROM shaping_shadow_diffs
         WHERE tenant_id=$1
         ORDER BY observed_at DESC
         LIMIT $2`, security.TenantID(ctx), limit)
	if err != nil {
		return nil, fmt.Errorf("query shaping shadow diffs: %w", err)
	}
	defer rows.Close()
	result := make([]shaping.ShadowDiff, 0)
	for rows.Next() {
		var item shaping.ShadowDiff
		if err := rows.Scan(&item.TenantID, &item.EventID, &item.ObservedAt, &item.ActiveConfig, &item.ActiveVersion, &item.ShadowConfig, &item.ShadowVersion, &item.ActiveKeep, &item.ShadowKeep, &item.ActiveRate, &item.ShadowRate, &item.ActiveEffects, &item.ShadowEffects); err != nil {
			return nil, fmt.Errorf("scan shaping shadow diff: %w", err)
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate shaping shadow diffs: %w", err)
	}
	return result, nil
}

// ShapingPruneResult reports compact decision/aggregate history removed by an
// explicit lifecycle operation.
type ShapingPruneResult struct {
	Decisions   int64 `json:"decisions"`
	MinuteStats int64 `json:"minute_stats"`
	ShadowDiffs int64 `json:"shadow_diffs"`
}

// PruneShapingBefore removes shaping evidence older than the requested cutoff
// for the authenticated tenant. Callers must enforce a horizon that is at
// least as long as the normal dedup/replay investigation window.
func (store *PostgresStore) PruneShapingBefore(ctx context.Context, before time.Time) (ShapingPruneResult, error) {
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return ShapingPruneResult{}, fmt.Errorf("begin shaping prune: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	tenantID := security.TenantID(ctx)
	result := ShapingPruneResult{}

	decisionTag, err := tx.Exec(ctx, `
        DELETE FROM shaping_decisions
         WHERE tenant_id=$1 AND observed_at < $2`, tenantID, before.UTC())
	if err != nil {
		return ShapingPruneResult{}, fmt.Errorf("prune shaping decisions: %w", err)
	}
	result.Decisions = decisionTag.RowsAffected()

	statTag, err := tx.Exec(ctx, `
        DELETE FROM shaping_minute_stats
         WHERE tenant_id=$1 AND bucket < $2`, tenantID, before.UTC())
	if err != nil {
		return ShapingPruneResult{}, fmt.Errorf("prune shaping minute statistics: %w", err)
	}
	result.MinuteStats = statTag.RowsAffected()

	diffTag, err := tx.Exec(ctx, `
        DELETE FROM shaping_shadow_diffs
         WHERE tenant_id=$1 AND observed_at < $2`, tenantID, before.UTC())
	if err != nil {
		return ShapingPruneResult{}, fmt.Errorf("prune shaping shadow differences: %w", err)
	}
	result.ShadowDiffs = diffTag.RowsAffected()

	if err := tx.Commit(ctx); err != nil {
		return ShapingPruneResult{}, fmt.Errorf("commit shaping prune: %w", err)
	}
	return result, nil
}
