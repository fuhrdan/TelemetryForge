package worker

import (
	"context"
	"errors"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/reliability"
)

// RoutingPlanner records durable destination intents for a processed event.
type RoutingPlanner interface {
	Process(context.Context, domain.Event) error
}

// RouterPlanner adapts the Telemetry Router planner into the worker pipeline.
type RouterPlanner struct{ planner RoutingPlanner }

func NewRouterPlanner(planner RoutingPlanner) (*RouterPlanner, error) {
	if planner == nil {
		return nil, errors.New("routing planner is required")
	}
	return &RouterPlanner{planner: planner}, nil
}

// Process fails transiently if the durable outbox cannot be recorded. Actual
// destination delivery never occurs here and therefore cannot block Kafka
// processing because one external backend is unavailable.
func (processor *RouterPlanner) Process(ctx context.Context, event domain.Event) (domain.Event, error) {
	if err := processor.planner.Process(ctx, event); err != nil {
		return domain.Event{}, reliability.New(reliability.Transient, "routing plan", err)
	}
	return event, nil
}
