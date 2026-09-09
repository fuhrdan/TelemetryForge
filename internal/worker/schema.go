package worker

import (
	"context"
	"log/slog"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/reliability"
	"github.com/fuhrdan/TelemetryForge/internal/schema"
)

// SchemaRegistry is the durable behavior required by the Schema Intelligence
// processor. Keeping the interface here prevents worker code from depending on
// PostgreSQL implementation details.
type SchemaRegistry interface {
	ObserveSchema(
		ctx context.Context,
		event domain.Event,
		description schema.Description,
	) (schema.RegistryEntry, []schema.Drift, error)
}

// SchemaInspector observes normalized, pre-policy telemetry shape.
type SchemaInspector struct {
	registry SchemaRegistry
	logger   *slog.Logger
	failOpen bool
}

// NewSchemaInspector creates the default v1.1 Schema Intelligence stage.
//
// Schema inspection is advisory by default, so registry failure must not turn
// into a production telemetry outage.
func NewSchemaInspector(registry SchemaRegistry) *SchemaInspector {
	return &SchemaInspector{registry: registry, failOpen: true}
}

// NewSchemaInspectorWithMode allows operators to opt into fail-closed schema
// registration while preserving fail-open as the product default.
func NewSchemaInspectorWithMode(
	registry SchemaRegistry,
	logger *slog.Logger,
	failOpen bool,
) *SchemaInspector {
	return &SchemaInspector{registry: registry, logger: logger, failOpen: failOpen}
}

// Process records one idempotent schema observation and returns the event
// unchanged.
//
// Drift findings never reject telemetry. A registry dependency failure is
// fail-open by default. If an operator explicitly configures fail-closed mode,
// the error is classified transient so the normal bounded worker retry policy
// applies. Retry is safe because schema_observations deduplicates by
// tenant/event ID.
func (inspector *SchemaInspector) Process(ctx context.Context, event domain.Event) (domain.Event, error) {
	if inspector == nil || inspector.registry == nil {
		return event, nil
	}
	description := schema.Describe(event)
	if _, _, err := inspector.registry.ObserveSchema(ctx, event, description); err != nil {
		if inspector.failOpen {
			if inspector.logger != nil {
				inspector.logger.Warn("schema intelligence observation failed open",
					"tenant_id", event.TenantID,
					"source", event.Source,
					"type", event.Type,
					"error", err)
			}
			return event, nil
		}
		return domain.Event{}, reliability.New(reliability.Transient, "schema intelligence", err)
	}
	return event, nil
}
