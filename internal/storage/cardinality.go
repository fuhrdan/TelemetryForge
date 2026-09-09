package storage

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/policy"
	"github.com/fuhrdan/TelemetryForge/internal/security"
)

const distributedCardinalityWindow = time.Hour

type distributedCardinalityTracker struct {
	store *PostgresStore
	mode  string
}

// NewDistributedCardinalityTracker returns a policy tracker backed by
// TimescaleDB/PostgreSQL shared state. All worker replicas using the same
// database therefore observe the same HLL register set for an hourly window.
func NewDistributedCardinalityTracker(store *PostgresStore, mode string) policy.CardinalityTracker {
	return &distributedCardinalityTracker{store: store, mode: mode}
}

func (tracker *distributedCardinalityTracker) ObserveCardinality(
	ctx context.Context,
	tenant, source, eventType, dimension, value string,
	now time.Time,
) (policy.Observation, error) {
	if tracker == nil || tracker.store == nil {
		return policy.Observation{}, fmt.Errorf("distributed cardinality store is unavailable")
	}
	if tracker.mode != "active" && tracker.mode != "shadow" {
		return policy.Observation{}, fmt.Errorf("unsupported cardinality mode %q", tracker.mode)
	}
	if tenant == "" {
		tenant = security.TenantID(ctx)
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	windowStart := now.Truncate(distributedCardinalityWindow)

	hash := policy.HashCardinalityValue(value)
	registerIndex, rank := policy.HLLRegister(hash)
	registers := make([]byte, 64)
	registers[registerIndex] = byte(rank)

	var persistedRegisters []byte
	var exactHashes []int64
	var exactOverflow bool
	var firstSeen time.Time
	var lastSeen time.Time
	var samples int64

	err := tracker.store.pool.QueryRow(ctx, `
		INSERT INTO cardinality_cluster_state
			(tenant_id, mode, source, event_type, dimension, window_start,
			 registers, exact_hashes, exact_overflow, first_seen, last_seen, samples)
		VALUES ($1,$2,$3,$4,$5,$6,$7,ARRAY[$8]::BIGINT[],FALSE,$9,$9,1)
		ON CONFLICT (tenant_id, mode, source, event_type, dimension, window_start)
		DO UPDATE SET
			registers = set_byte(
				cardinality_cluster_state.registers,
				$10,
				GREATEST(get_byte(cardinality_cluster_state.registers, $10), $11)
			),
			exact_hashes = CASE
				WHEN cardinality_cluster_state.exact_overflow THEN cardinality_cluster_state.exact_hashes
				WHEN $8 = ANY(cardinality_cluster_state.exact_hashes) THEN cardinality_cluster_state.exact_hashes
				WHEN cardinality(cardinality_cluster_state.exact_hashes) < 16
					THEN array_append(cardinality_cluster_state.exact_hashes, $8)
				ELSE cardinality_cluster_state.exact_hashes
			END,
			exact_overflow = cardinality_cluster_state.exact_overflow OR (
				NOT ($8 = ANY(cardinality_cluster_state.exact_hashes))
				AND cardinality(cardinality_cluster_state.exact_hashes) >= 16
			),
			first_seen = LEAST(cardinality_cluster_state.first_seen, $9),
			last_seen = GREATEST(cardinality_cluster_state.last_seen, $9),
			samples = cardinality_cluster_state.samples + 1
		RETURNING registers, exact_hashes, exact_overflow, first_seen, last_seen, samples`,
		tenant, tracker.mode, source, eventType, dimension, windowStart,
		registers, int64(hash), now, registerIndex, int(rank),
	).Scan(
		&persistedRegisters,
		&exactHashes,
		&exactOverflow,
		&firstSeen,
		&lastSeen,
		&samples,
	)
	if err != nil {
		return policy.Observation{}, fmt.Errorf("update distributed cardinality state: %w", err)
	}

	observed := policy.EstimateState(persistedRegisters, len(exactHashes), exactOverflow)
	projected := policy.ProjectOneHour(observed, uint64(maxInt64(samples, 0)), firstSeen, now)

	return policy.Observation{
		ObservedUnique:  observed,
		ProjectedUnique: projected,
		FirstSeen:       firstSeen,
		LastSeen:        lastSeen,
		WindowStart:     windowStart,
		Samples:         uint64(maxInt64(samples, 0)),
		Fingerprint:     policy.FingerprintHash(hash),
	}, nil
}

// RecordCardinalityBudgetStatus upserts the current status for one versioned
// policy budget. The policy engine rate-limits these updates; cluster HLL state
// itself is updated on every observation.
func (store *PostgresStore) RecordCardinalityBudgetStatus(
	ctx context.Context,
	status policy.BudgetStatus,
) error {
	_, err := store.pool.Exec(ctx, `
		INSERT INTO cardinality_budget_status
			(tenant_id, policy_name, policy_version, mode, budget_name,
			 source_pattern, event_type_pattern, window_start, series_limit,
			 observed_unique, projected_unique, consumption_percent, status, observed_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		ON CONFLICT (tenant_id, policy_name, policy_version, mode, budget_name)
		DO UPDATE SET
			source_pattern = EXCLUDED.source_pattern,
			event_type_pattern = EXCLUDED.event_type_pattern,
			window_start = EXCLUDED.window_start,
			series_limit = EXCLUDED.series_limit,
			observed_unique = EXCLUDED.observed_unique,
			projected_unique = EXCLUDED.projected_unique,
			consumption_percent = EXCLUDED.consumption_percent,
			status = EXCLUDED.status,
			observed_at = EXCLUDED.observed_at`,
		tenantForPolicyFinding(ctx, status.TenantID),
		status.PolicyName, status.PolicyVersion, status.Mode, status.BudgetName,
		status.Source, status.EventType, status.WindowStart, status.SeriesLimit,
		status.ObservedUnique, status.ProjectedUnique, status.ConsumptionPercent,
		status.Status, status.ObservedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert cardinality budget status: %w", err)
	}
	return nil
}

// ListCardinalityBudgetStatus returns current tenant budget consumption.
func (store *PostgresStore) ListCardinalityBudgetStatus(
	ctx context.Context,
	limit int,
) ([]policy.BudgetStatus, error) {
	if limit < 1 || limit > 500 {
		limit = 100
	}
	rows, err := store.pool.Query(ctx, `
		SELECT tenant_id, policy_name, policy_version, mode, budget_name,
		       source_pattern, event_type_pattern, window_start, series_limit,
		       observed_unique, projected_unique, consumption_percent, status, observed_at
		  FROM cardinality_budget_status
		 WHERE tenant_id = $1
		   AND observed_at >= now() - interval '2 hours'
		 ORDER BY CASE status
		            WHEN 'exceeded' THEN 4
		            WHEN 'critical' THEN 3
		            WHEN 'warning' THEN 2
		            ELSE 1
		          END DESC,
		          consumption_percent DESC,
		          observed_at DESC
		 LIMIT $2`, security.TenantID(ctx), limit)
	if err != nil {
		return nil, fmt.Errorf("query cardinality budget status: %w", err)
	}
	defer rows.Close()

	result := make([]policy.BudgetStatus, 0, limit)
	for rows.Next() {
		var item policy.BudgetStatus
		if err := rows.Scan(
			&item.TenantID, &item.PolicyName, &item.PolicyVersion, &item.Mode,
			&item.BudgetName, &item.Source, &item.EventType, &item.WindowStart,
			&item.SeriesLimit, &item.ObservedUnique, &item.ProjectedUnique,
			&item.ConsumptionPercent, &item.Status, &item.ObservedAt,
		); err != nil {
			return nil, fmt.Errorf("scan cardinality budget status: %w", err)
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

// ListDistributedCardinalityStates returns the current shared hourly state,
// ordered by projected growth. Internal budget-aggregation pseudo-sources are
// hidden because budget consumption has its own API.
func (store *PostgresStore) ListDistributedCardinalityStates(
	ctx context.Context,
	mode string,
	limit int,
) ([]policy.DistributedState, error) {
	if mode != "active" && mode != "shadow" {
		mode = "active"
	}
	if limit < 1 || limit > 500 {
		limit = 100
	}
	windowStart := time.Now().UTC().Truncate(distributedCardinalityWindow)
	fetchLimit := limit * 20
	if fetchLimit < 500 {
		fetchLimit = 500
	}
	if fetchLimit > 5000 {
		fetchLimit = 5000
	}
	rows, err := store.pool.Query(ctx, `
		SELECT tenant_id, mode, source, event_type, dimension, window_start,
		       registers, exact_hashes, exact_overflow, first_seen, last_seen, samples
		  FROM cardinality_cluster_state
		 WHERE tenant_id = $1
		   AND mode = $2
		   AND window_start = $3
		   AND source <> '__budget__'
		 ORDER BY last_seen DESC
		 LIMIT $4`, security.TenantID(ctx), mode, windowStart, fetchLimit)
	if err != nil {
		return nil, fmt.Errorf("query distributed cardinality state: %w", err)
	}
	defer rows.Close()

	result := make([]policy.DistributedState, 0, limit)
	for rows.Next() {
		var item policy.DistributedState
		var registers []byte
		var exactHashes []int64
		var exactOverflow bool
		var samples int64
		if err := rows.Scan(
			&item.TenantID, &item.Mode, &item.Source, &item.EventType,
			&item.Dimension, &item.WindowStart, &registers, &exactHashes,
			&exactOverflow, &item.FirstSeen, &item.LastSeen, &samples,
		); err != nil {
			return nil, fmt.Errorf("scan distributed cardinality state: %w", err)
		}
		item.Samples = uint64(maxInt64(samples, 0))
		item.ObservedUnique = policy.EstimateState(registers, len(exactHashes), exactOverflow)
		item.ProjectedUnique = policy.ProjectOneHour(
			item.ObservedUnique, item.Samples, item.FirstSeen, item.LastSeen,
		)
		elapsedMinutes := item.LastSeen.Sub(item.FirstSeen).Minutes()
		if elapsedMinutes >= 1 {
			item.GrowthPerMinute = float64(item.ObservedUnique) / elapsedMinutes
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// The query limits I/O to a bounded recent candidate set, then Go sorts by
	// the computed projection because the HLL estimate is intentionally computed
	// outside PostgreSQL.
	sortDistributedStates(result)
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func sortDistributedStates(states []policy.DistributedState) {
	sort.SliceStable(states, func(left, right int) bool {
		leftGrowth := int64(states[left].ProjectedUnique) - int64(states[left].ObservedUnique)
		rightGrowth := int64(states[right].ProjectedUnique) - int64(states[right].ObservedUnique)
		if leftGrowth == rightGrowth {
			return states[left].ProjectedUnique > states[right].ProjectedUnique
		}
		return leftGrowth > rightGrowth
	})
}

func maxInt64(value, minimum int64) int64 {
	if value < minimum {
		return minimum
	}
	return value
}
