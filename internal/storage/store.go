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
	"github.com/fuhrdan/TelemetryForge/internal/incidentarchive"
	"github.com/fuhrdan/TelemetryForge/internal/lifecycle"
	"github.com/fuhrdan/TelemetryForge/internal/policy"
	"github.com/fuhrdan/TelemetryForge/internal/replay"
	"github.com/fuhrdan/TelemetryForge/internal/router"
	"github.com/fuhrdan/TelemetryForge/internal/schema"
	"github.com/fuhrdan/TelemetryForge/internal/shaping"
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
	ListDistributedCardinalityStates(ctx context.Context, mode string, limit int) ([]policy.DistributedState, error)
	ListCardinalityBudgetStatus(ctx context.Context, limit int) ([]policy.BudgetStatus, error)
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

// SchemaReader exposes tenant-scoped Schema Intelligence state.
type SchemaReader interface {
	ListSchemas(ctx context.Context, limit int) ([]schema.RegistryEntry, error)
	SchemaHistory(ctx context.Context, source, eventType string) ([]schema.RegistryEntry, error)
	ListSchemaDrifts(ctx context.Context, limit int) ([]schema.Drift, error)
	SchemaDiff(ctx context.Context, source, eventType, fromVersion, toVersion string) (schema.VersionDiff, error)
}

// RoutingReader exposes tenant-scoped Telemetry Router state.
type RoutingReader interface {
	ListRoutingDeliveries(ctx context.Context, status string, limit int) ([]router.Delivery, error)
	ListRoutingShadowDiffs(ctx context.Context, limit int) ([]router.ShadowDiff, error)
	ListRoutingDeadLetters(ctx context.Context, destination string, limit int) ([]router.DeadLetter, error)
	ListRoutingDestinationHealth(ctx context.Context, limit int) ([]router.DestinationHealth, error)
}

// ShapingReader exposes tenant-scoped adaptive sampling/shaping evidence.
type ShapingReader interface {
	ShapingSummary(ctx context.Context, from time.Time) (shaping.Summary, error)
	ListShapingStats(ctx context.Context, from time.Time, limit int) ([]shaping.Stat, error)
	ListShapingShadowDiffs(ctx context.Context, limit int) ([]shaping.ShadowDiff, error)
}

// IncidentArchiveReader exposes tenant-scoped portable archive import
// provenance. Archive file bytes are deliberately not stored in PostgreSQL.
type IncidentArchiveReader interface {
	ListIncidentArchiveImports(
		ctx context.Context,
		limit int,
	) ([]incidentarchive.ImportProvenance, error)
}

// LifecycleStore is the global configuration control-plane contract.
type LifecycleStore interface {
    CreateLifecycleArtifact(context.Context, lifecycle.Kind, []byte, string) (lifecycle.Artifact, error)
    ListLifecycleArtifacts(context.Context, lifecycle.Kind, int) ([]lifecycle.Artifact, error)
    LifecycleArtifact(context.Context, string, bool) (lifecycle.Artifact, error)
    SetLifecycleShadow(context.Context, string, string) error
    ApproveLifecycle(context.Context, string, string, string) error
    AddLifecycleEvidence(context.Context, string, string, string, string, string, string, string) (lifecycle.Evidence, error)
    ScheduleLifecycle(context.Context, string, time.Time, string) error
    ActivateLifecycle(context.Context, string, string) error
    RollbackLifecycle(context.Context, string, string) error
    RetireLifecycle(context.Context, string, string) error
    ManagedLifecycleConfig(context.Context, lifecycle.Kind, string) (lifecycle.Artifact, bool, error)
    RecordLifecycleRuntimeLoad(context.Context, lifecycle.RuntimeLoad) error
    LifecycleConvergence(context.Context) ([]lifecycle.Convergence, error)
    ListLifecycleAudit(context.Context, int) ([]lifecycle.AuditEntry, error)
}
