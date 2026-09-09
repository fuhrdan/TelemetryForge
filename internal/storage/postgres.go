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
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore persists telemetry in PostgreSQL/TimescaleDB.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore opens a connection pool and verifies database connectivity.
func NewPostgresStore(ctx context.Context, databaseURL string) (*PostgresStore, error) {
	if databaseURL == "" {
		return nil, errors.New("database URL is required")
	}

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}

	config.MaxConns = 10
	config.MinConns = 1
	config.MaxConnLifetime = 30 * time.Minute
	config.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create PostgreSQL pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping PostgreSQL: %w", err)
	}

	return &PostgresStore{pool: pool}, nil
}

// WriteEvent stores one event exactly once by canonical event ID.
//
// TimescaleDB requires unique indexes on hypertables to include the partition
// column. TelemetryForge therefore keeps tenant-scoped event-ID uniqueness in
// the ordinary `event_dedup` PostgreSQL table and writes the hypertable row in
// the same transaction. A duplicate Kafka delivery becomes a safe no-op without
// allowing one tenant's event ID to suppress another tenant's event.
func (store *PostgresStore) WriteEvent(ctx context.Context, event domain.Event) error {
	tags, err := json.Marshal(event.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}

	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin event transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	result, err := tx.Exec(ctx,
		`INSERT INTO event_dedup (tenant_id, event_id, first_seen_at)
		 VALUES ($1, $2, now())
		 ON CONFLICT (tenant_id, event_id) DO NOTHING`,
		tenantForEvent(ctx, event), event.ID,
	)
	if err != nil {
		return fmt.Errorf("reserve event id: %w", err)
	}

	if result.RowsAffected() == 0 {
		return nil
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO telemetry_events
			(tenant_id, event_id, source, event_type, event_time, tags, payload,
			 metric_value, metric_unit, schema_version, correlation_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		tenantForEvent(ctx, event),
		event.ID,
		event.Source,
		event.Type,
		event.Timestamp.UTC(),
		string(tags),
		nullableJSON(event.Payload),
		event.Value,
		nullIfEmpty(event.Unit),
		event.SchemaVersion,
		nullIfEmpty(event.CorrelationID),
	)
	if err != nil {
		return fmt.Errorf("insert telemetry event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit event transaction: %w", err)
	}
	return nil
}

// QueryEvents returns newest-first telemetry matching the supplied filters.
func (store *PostgresStore) QueryEvents(ctx context.Context, query Query) ([]domain.Event, error) {
	limit := query.Limit
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	rows, err := store.pool.Query(ctx, `
		SELECT tenant_id, event_id, source, event_type, event_time, tags, payload,
		       metric_value, COALESCE(metric_unit, ''), schema_version,
		       COALESCE(correlation_id, '')
		  FROM telemetry_events
		 WHERE tenant_id = $1
		   AND ($2 = '' OR source = $2)
		   AND ($3 = '' OR event_type = $3)
		   AND ($4::timestamptz IS NULL OR event_time >= $4)
		   AND ($5::timestamptz IS NULL OR event_time <= $5)
		   AND ($6 = FALSE OR metric_value IS NOT NULL)
		 ORDER BY event_time DESC
		 LIMIT $7`,
		queryTenant(ctx, query.TenantID), query.Source, query.Type,
		nullableTime(query.From), nullableTime(query.To), query.MetricsOnly, limit)
	if err != nil {
		return nil, fmt.Errorf("query telemetry events: %w", err)
	}
	defer rows.Close()

	events := make([]domain.Event, 0, limit)
	for rows.Next() {
		var event domain.Event
		var tags []byte
		var payload []byte
		if err := rows.Scan(
			&event.TenantID, &event.ID, &event.Source, &event.Type, &event.Timestamp, &tags, &payload,
			&event.Value, &event.Unit, &event.SchemaVersion, &event.CorrelationID,
		); err != nil {
			return nil, fmt.Errorf("scan telemetry event: %w", err)
		}
		if len(tags) > 0 {
			if err := json.Unmarshal(tags, &event.Tags); err != nil {
				return nil, fmt.Errorf("decode event tags: %w", err)
			}
		}
		if len(payload) > 0 {
			event.Payload = append(json.RawMessage(nil), payload...)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate telemetry events: %w", err)
	}
	return events, nil
}

// PruneDedupBefore removes idempotency reservations older than the supplied
// cutoff. Operators should keep this horizon longer than raw telemetry
// retention and any Kafka/replay window so an old replay cannot recreate data
// that should still be considered already processed.
func (store *PostgresStore) PruneDedupBefore(ctx context.Context, before time.Time) (int64, error) {
	if before.IsZero() {
		return 0, errors.New("deduplication cutoff is required")
	}
	result, err := store.pool.Exec(ctx,
		`DELETE FROM event_dedup
		  WHERE tenant_id = $1
		    AND first_seen_at < $2`,
		security.TenantID(ctx), before.UTC())
	if err != nil {
		return 0, fmt.Errorf("prune event deduplication rows: %w", err)
	}
	return result.RowsAffected(), nil
}

// Ready verifies that the database can answer a lightweight request.
func (store *PostgresStore) Ready(ctx context.Context) error {
	return store.pool.Ping(ctx)
}

// Close releases all database connections.
func (store *PostgresStore) Close() {
	store.pool.Close()
}

func tenantForEvent(ctx context.Context, event domain.Event) string {
	if tenant := strings.TrimSpace(event.TenantID); tenant != "" {
		return tenant
	}
	return security.TenantID(ctx)
}

func queryTenant(ctx context.Context, configured string) string {
	// An authenticated/trusted principal always wins. Query structs cannot be
	// used to escape the authorization boundary by supplying another tenant.
	if principal, ok := security.PrincipalFrom(ctx); ok {
		if tenant := strings.TrimSpace(principal.TenantID); tenant != "" {
			return tenant
		}
	}
	if tenant := strings.TrimSpace(configured); tenant != "" {
		return tenant
	}
	return "default"
}

func nullableJSON(value json.RawMessage) any {
	if len(value) == 0 {
		return nil
	}
	return string(value)
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value.UTC()
}
