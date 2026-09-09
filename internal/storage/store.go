// Package storage owns durable TelemetryForge persistence.
//
// PostgreSQL stores relational/idempotency metadata while TimescaleDB stores
// the time-series event stream. Interfaces in this package keep SQL concerns
// out of HTTP and Kafka transport code.
package storage

import (
	"context"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/costsim"
	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/evidence"
	"github.com/fuhrdan/TelemetryForge/internal/replay"
)

// Query describes a bounded telemetry lookup.
type Query struct {
	TenantID    string
	Source      string
	Type        string
	From        time.Time
	To          time.Time
	Limit       int
	MetricsOnly bool
}

// Writer persists canonical telemetry events.
type Writer interface {
	WriteEvent(ctx context.Context, event domain.Event) error
}

// Reader retrieves canonical telemetry events.
type Reader interface {
	QueryEvents(ctx context.Context, query Query) ([]domain.Event, error)
}

// PolicyReader exposes Cardinality Firewall and shadow-policy evidence.
type PolicyReader interface {
	ListCardinalityFindings(ctx context.Context, limit int) ([]CardinalityFinding, error)
	ListPolicyDiffs(ctx context.Context, limit int) ([]PolicyDiff, error)
}

// ReplayReader exposes incident replay and cost-simulation history.
type ReplayReader interface {
	ListReplayRuns(ctx context.Context, limit int) ([]replay.Run, error)
	ListCostSimulations(ctx context.Context, limit int) ([]costsim.Result, error)
}

// EvidenceStore persists/regenerates the latest incident Evidence Graph.
type EvidenceStore interface {
	SaveEvidenceGraph(ctx context.Context, graph evidence.Graph) error
	EvidenceGraph(ctx context.Context, incidentID string) (evidence.Graph, bool, error)
}
