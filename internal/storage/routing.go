package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/router"
	"github.com/fuhrdan/TelemetryForge/internal/security"
	"github.com/jackc/pgx/v5"
)

// EnqueueRoutingPlan persists deduplicated destination intents for one event.
// Retried worker execution is safe because tenant/event/destination is unique.
func (store *PostgresStore) EnqueueRoutingPlan(ctx context.Context, event domain.Event, decisions []router.Decision) error {
	if len(decisions) == 0 {
		return nil
	}
	envelope, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode routing envelope: %w", err)
	}

	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin routing plan: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	tenantID := tenantForEvent(ctx, event)
	for _, decision := range decisions {
		_, err = tx.Exec(ctx, `
            INSERT INTO routing_deliveries
                (tenant_id,event_id,destination,route_rules,envelope,status,
                 attempts,max_attempts,base_delay_ms,max_delay_ms,failure_fallback,
                 next_attempt_at,created_at,updated_at)
            VALUES ($1,$2,$3,$4,$5::jsonb,'pending',0,$6,$7,$8,$9,now(),now(),now())
            ON CONFLICT (tenant_id,event_id,destination) DO NOTHING`,
			tenantID, event.ID, decision.Destination, decision.RouteRules,
			string(envelope), decision.MaxAttempts, decision.BaseDelayMS,
			decision.MaxDelayMS, decision.FailureFallback)
		if err != nil {
			return fmt.Errorf("enqueue routing destination %q: %w", decision.Destination, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit routing plan: %w", err)
	}
	return nil
}

// RecordRoutingShadowDiff stores candidate destination-set differences only.
func (store *PostgresStore) RecordRoutingShadowDiff(ctx context.Context, diff router.ShadowDiff) error {
	tenantID := diff.TenantID
	if tenantID == "" {
		tenantID = security.TenantID(ctx)
	}
	_, err := store.pool.Exec(ctx, `
        INSERT INTO routing_shadow_diffs
            (tenant_id,event_id,observed_at,active_config,active_version,
             shadow_config,shadow_version,active_destinations,shadow_destinations,
             added,removed)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
        ON CONFLICT (tenant_id,event_id) DO UPDATE SET
            observed_at=EXCLUDED.observed_at,
            active_config=EXCLUDED.active_config,
            active_version=EXCLUDED.active_version,
            shadow_config=EXCLUDED.shadow_config,
            shadow_version=EXCLUDED.shadow_version,
            active_destinations=EXCLUDED.active_destinations,
            shadow_destinations=EXCLUDED.shadow_destinations,
            added=EXCLUDED.added,
            removed=EXCLUDED.removed`,
		tenantID, diff.EventID, diff.ObservedAt, diff.ActiveConfig, diff.ActiveVersion,
		diff.ShadowConfig, diff.ShadowVersion, diff.ActiveDestinations,
		diff.ShadowDestinations, diff.Added, diff.Removed)
	if err != nil {
		return fmt.Errorf("persist routing shadow diff: %w", err)
	}
	return nil
}

// ClaimRoutingDelivery atomically leases one due row for one destination.
func (store *PostgresStore) ClaimRoutingDelivery(ctx context.Context, destination string, lease time.Duration) (router.Delivery, bool, error) {
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return router.Delivery{}, false, fmt.Errorf("begin routing claim: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	var delivery router.Delivery
	var raw []byte
	var leaseUntil time.Time
	err = tx.QueryRow(ctx, `
        SELECT tenant_id,event_id,destination,route_rules,envelope,status,
               attempts,max_attempts,base_delay_ms,max_delay_ms,failure_fallback,
               next_attempt_at,created_at,updated_at
          FROM routing_deliveries
         WHERE destination=$1
           AND (
                (status IN ('pending','retry') AND next_attempt_at <= now())
                OR (status='sending' AND lease_until < now())
           )
         ORDER BY created_at ASC
         FOR UPDATE SKIP LOCKED
         LIMIT 1`, destination).Scan(
		&delivery.TenantID, &delivery.EventID, &delivery.Destination, &delivery.RouteRules,
		&raw, &delivery.Status, &delivery.Attempts, &delivery.MaxAttempts,
		&delivery.BaseDelayMS, &delivery.MaxDelayMS, &delivery.FailureFallback,
		&delivery.NextAttemptAt, &delivery.CreatedAt, &delivery.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return router.Delivery{}, false, nil
		}
		return router.Delivery{}, false, fmt.Errorf("select routing delivery: %w", err)
	}
	if err := json.Unmarshal(raw, &delivery.Event); err != nil {
		return router.Delivery{}, false, fmt.Errorf("decode routing envelope: %w", err)
	}
	leaseUntil = time.Now().UTC().Add(lease)
	delivery.Attempts++
	delivery.Status = "sending"
	delivery.LeaseUntil = &leaseUntil
	_, err = tx.Exec(ctx, `
        UPDATE routing_deliveries
           SET status='sending', attempts=$4, lease_until=$5, updated_at=now()
         WHERE tenant_id=$1 AND event_id=$2 AND destination=$3`,
		delivery.TenantID, delivery.EventID, delivery.Destination, delivery.Attempts, leaseUntil)
	if err != nil {
		return router.Delivery{}, false, fmt.Errorf("lease routing delivery: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return router.Delivery{}, false, fmt.Errorf("commit routing claim: %w", err)
	}
	return delivery, true, nil
}

func (store *PostgresStore) MarkRoutingDelivered(ctx context.Context, delivery router.Delivery) error {
	_, err := store.pool.Exec(ctx, `
        UPDATE routing_deliveries
           SET status='delivered', lease_until=NULL, last_error='', delivered_at=now(), updated_at=now()
         WHERE tenant_id=$1 AND event_id=$2 AND destination=$3`,
		delivery.TenantID, delivery.EventID, delivery.Destination)
	if err != nil {
		return fmt.Errorf("mark routing delivery delivered: %w", err)
	}
	return nil
}

func (store *PostgresStore) MarkRoutingRetry(ctx context.Context, delivery router.Delivery, message string, next time.Time) error {
	_, err := store.pool.Exec(ctx, `
        UPDATE routing_deliveries
           SET status='retry', lease_until=NULL, last_error=$4,
               next_attempt_at=$5, updated_at=now()
         WHERE tenant_id=$1 AND event_id=$2 AND destination=$3`,
		delivery.TenantID, delivery.EventID, delivery.Destination, message, next.UTC())
	if err != nil {
		return fmt.Errorf("mark routing delivery retry: %w", err)
	}
	return nil
}

// DeadLetterRoutingDelivery atomically terminates one failed destination and,
// when configured, inserts a new delivery intent for its fallback destination.
func (store *PostgresStore) DeadLetterRoutingDelivery(ctx context.Context, delivery router.Delivery, message string, fallback *router.Decision) error {
	envelope, err := json.Marshal(delivery.Event)
	if err != nil {
		return fmt.Errorf("encode routing dead letter: %w", err)
	}
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin routing dead letter: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	_, err = tx.Exec(ctx, `
        INSERT INTO routing_dead_letters
            (tenant_id,event_id,destination,route_rules,envelope,attempts,error,failed_at,failure_count)
        VALUES ($1,$2,$3,$4,$5::jsonb,$6,$7,now(),1)
        ON CONFLICT (tenant_id,event_id,destination) DO UPDATE SET
            route_rules=EXCLUDED.route_rules,envelope=EXCLUDED.envelope,
            attempts=EXCLUDED.attempts,error=EXCLUDED.error,failed_at=EXCLUDED.failed_at,
            failure_count=routing_dead_letters.failure_count+1`,
		delivery.TenantID, delivery.EventID, delivery.Destination, delivery.RouteRules,
		string(envelope), delivery.Attempts, message)
	if err != nil {
		return fmt.Errorf("insert routing dead letter: %w", err)
	}

	_, err = tx.Exec(ctx, `
        UPDATE routing_deliveries
           SET status='dead_letter', lease_until=NULL, last_error=$4, updated_at=now()
         WHERE tenant_id=$1 AND event_id=$2 AND destination=$3`,
		delivery.TenantID, delivery.EventID, delivery.Destination, message)
	if err != nil {
		return fmt.Errorf("mark routing dead letter: %w", err)
	}

	if fallback != nil {
		_, err = tx.Exec(ctx, `
            INSERT INTO routing_deliveries
                (tenant_id,event_id,destination,route_rules,envelope,status,
                 attempts,max_attempts,base_delay_ms,max_delay_ms,failure_fallback,
                 next_attempt_at,created_at,updated_at)
            VALUES ($1,$2,$3,$4,$5::jsonb,'pending',0,$6,$7,$8,$9,now(),now(),now())
            ON CONFLICT (tenant_id,event_id,destination) DO NOTHING`,
			delivery.TenantID, delivery.EventID, fallback.Destination, fallback.RouteRules,
			string(envelope), fallback.MaxAttempts, fallback.BaseDelayMS,
			fallback.MaxDelayMS, fallback.FailureFallback)
		if err != nil {
			return fmt.Errorf("enqueue routing failure fallback: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit routing dead letter: %w", err)
	}
	return nil
}

func (store *PostgresStore) RecordRoutingDestinationHealth(ctx context.Context, tenantID, destination string, success bool, message string) error {
	if tenantID == "" {
		tenantID = security.TenantID(ctx)
	}
	if success {
		_, err := store.pool.Exec(ctx, `
            INSERT INTO routing_destination_health
                (tenant_id,destination,state,consecutive_failures,last_success_at,last_error,updated_at)
            VALUES ($1,$2,'healthy',0,now(),'',now())
            ON CONFLICT (tenant_id,destination) DO UPDATE SET
                state='healthy',consecutive_failures=0,last_success_at=now(),last_error='',updated_at=now()`,
			tenantID, destination)
		if err != nil {
			return fmt.Errorf("record destination success: %w", err)
		}
		return nil
	}
	_, err := store.pool.Exec(ctx, `
        INSERT INTO routing_destination_health
            (tenant_id,destination,state,consecutive_failures,last_failure_at,last_error,updated_at)
        VALUES ($1,$2,'degraded',1,now(),$3,now())
        ON CONFLICT (tenant_id,destination) DO UPDATE SET
            consecutive_failures=routing_destination_health.consecutive_failures+1,
            state=CASE WHEN routing_destination_health.consecutive_failures+1 >= 5 THEN 'unhealthy' ELSE 'degraded' END,
            last_failure_at=now(),last_error=$3,updated_at=now()`,
		tenantID, destination, message)
	if err != nil {
		return fmt.Errorf("record destination failure: %w", err)
	}
	return nil
}

func (store *PostgresStore) ListRoutingDeliveries(ctx context.Context, status string, limit int) ([]router.Delivery, error) {
	if limit < 1 || limit > 500 {
		limit = 100
	}
	rows, err := store.pool.Query(ctx, `
        SELECT tenant_id,event_id,destination,route_rules,envelope,status,attempts,
               max_attempts,base_delay_ms,max_delay_ms,failure_fallback,next_attempt_at,
               lease_until,last_error,created_at,updated_at,delivered_at
          FROM routing_deliveries
         WHERE tenant_id=$1 AND ($2='' OR status=$2)
         ORDER BY updated_at DESC
         LIMIT $3`, security.TenantID(ctx), status, limit)
	if err != nil {
		return nil, fmt.Errorf("query routing deliveries: %w", err)
	}
	defer rows.Close()
	result := make([]router.Delivery, 0)
	for rows.Next() {
		var item router.Delivery
		var raw []byte
		if err := rows.Scan(&item.TenantID, &item.EventID, &item.Destination, &item.RouteRules,
			&raw, &item.Status, &item.Attempts, &item.MaxAttempts, &item.BaseDelayMS, &item.MaxDelayMS,
			&item.FailureFallback, &item.NextAttemptAt, &item.LeaseUntil, &item.LastError,
			&item.CreatedAt, &item.UpdatedAt, &item.DeliveredAt); err != nil {
			return nil, fmt.Errorf("scan routing delivery: %w", err)
		}
		if err := json.Unmarshal(raw, &item.Event); err != nil {
			return nil, fmt.Errorf("decode routing delivery envelope: %w", err)
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (store *PostgresStore) ListRoutingShadowDiffs(ctx context.Context, limit int) ([]router.ShadowDiff, error) {
	if limit < 1 || limit > 500 {
		limit = 100
	}
	rows, err := store.pool.Query(ctx, `
        SELECT tenant_id,event_id,observed_at,active_config,active_version,
               shadow_config,shadow_version,active_destinations,shadow_destinations,added,removed
          FROM routing_shadow_diffs
         WHERE tenant_id=$1
         ORDER BY observed_at DESC LIMIT $2`, security.TenantID(ctx), limit)
	if err != nil {
		return nil, fmt.Errorf("query routing shadow diffs: %w", err)
	}
	defer rows.Close()
	result := make([]router.ShadowDiff, 0)
	for rows.Next() {
		var d router.ShadowDiff
		if err := rows.Scan(&d.TenantID, &d.EventID, &d.ObservedAt, &d.ActiveConfig, &d.ActiveVersion, &d.ShadowConfig, &d.ShadowVersion, &d.ActiveDestinations, &d.ShadowDestinations, &d.Added, &d.Removed); err != nil {
			return nil, err
		}
		result = append(result, d)
	}
	return result, rows.Err()
}

func (store *PostgresStore) ListRoutingDeadLetters(ctx context.Context, destination string, limit int) ([]router.DeadLetter, error) {
	if limit < 1 || limit > 500 {
		limit = 100
	}
	rows, err := store.pool.Query(ctx, `
        SELECT tenant_id,event_id,destination,route_rules,envelope,attempts,error,failed_at
          FROM routing_dead_letters
         WHERE tenant_id=$1 AND ($2='' OR destination=$2)
         ORDER BY failed_at DESC LIMIT $3`, security.TenantID(ctx), destination, limit)
	if err != nil {
		return nil, fmt.Errorf("query routing dead letters: %w", err)
	}
	defer rows.Close()
	result := make([]router.DeadLetter, 0)
	for rows.Next() {
		var d router.DeadLetter
		var raw []byte
		if err := rows.Scan(&d.TenantID, &d.EventID, &d.Destination, &d.RouteRules, &raw, &d.Attempts, &d.Error, &d.FailedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &d.Event); err != nil {
			return nil, err
		}
		result = append(result, d)
	}
	return result, rows.Err()
}

func (store *PostgresStore) RequeueRoutingDeadLetter(ctx context.Context, eventID, destination string) error {
	tenantID := security.TenantID(ctx)
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	tag, err := tx.Exec(ctx, `
        UPDATE routing_deliveries SET status='retry',attempts=0,next_attempt_at=now(),
               lease_until=NULL,last_error='',updated_at=now()
         WHERE tenant_id=$1 AND event_id=$2 AND destination=$3 AND status='dead_letter'`, tenantID, eventID, destination)
	if err != nil {
		return fmt.Errorf("requeue routing delivery: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("routing dead letter not found")
	}
	if _, err := tx.Exec(ctx, `DELETE FROM routing_dead_letters WHERE tenant_id=$1 AND event_id=$2 AND destination=$3`, tenantID, eventID, destination); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (store *PostgresStore) ListRoutingDestinationHealth(ctx context.Context, limit int) ([]router.DestinationHealth, error) {
	if limit < 1 || limit > 500 {
		limit = 100
	}
	rows, err := store.pool.Query(ctx, `
        WITH names AS (
            SELECT destination FROM routing_deliveries WHERE tenant_id=$1
            UNION SELECT destination FROM routing_destination_health WHERE tenant_id=$1
            UNION SELECT destination FROM routing_dead_letters WHERE tenant_id=$1
        )
        SELECT n.destination,
               COALESCE(h.state,'unknown'),COALESCE(h.consecutive_failures,0),
               h.last_success_at,h.last_failure_at,COALESCE(h.last_error,''),
               COALESCE(h.updated_at,now()),
               COUNT(*) FILTER (WHERE d.status='pending'),
               COUNT(*) FILTER (WHERE d.status='retry'),
               (SELECT COUNT(*) FROM routing_dead_letters dl WHERE dl.tenant_id=$1 AND dl.destination=n.destination)
          FROM names n
          LEFT JOIN routing_destination_health h ON h.tenant_id=$1 AND h.destination=n.destination
          LEFT JOIN routing_deliveries d ON d.tenant_id=$1 AND d.destination=n.destination
         GROUP BY n.destination,h.state,h.consecutive_failures,h.last_success_at,h.last_failure_at,h.last_error,h.updated_at
         ORDER BY n.destination LIMIT $2`, security.TenantID(ctx), limit)
	if err != nil {
		return nil, fmt.Errorf("query routing destination health: %w", err)
	}
	defer rows.Close()
	result := make([]router.DestinationHealth, 0)
	for rows.Next() {
		var h router.DestinationHealth
		h.TenantID = security.TenantID(ctx)
		if err := rows.Scan(&h.Destination, &h.State, &h.ConsecutiveFailures, &h.LastSuccessAt, &h.LastFailureAt, &h.LastError, &h.UpdatedAt, &h.Pending, &h.Retrying, &h.DeadLetters); err != nil {
			return nil, err
		}
		result = append(result, h)
	}
	return result, rows.Err()
}
