package worker

import (
	"context"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/reliability"
)

// PolicyEngine is the active/shadow telemetry-policy behavior required by the
// worker pipeline.
type PolicyEngine interface {
	Evaluate(ctx context.Context, event domain.Event) (domain.Event, error)
}

// PolicyProcessor adapts policy evaluation to the worker processor chain.
type PolicyProcessor struct {
	engine PolicyEngine
}

// NewPolicyProcessor creates a policy stage.
func NewPolicyProcessor(engine PolicyEngine) *PolicyProcessor {
	return &PolicyProcessor{engine: engine}
}

// Process applies the active policy and records shadow-policy evidence.
//
// This stage runs after the Flight Recorder and normalization but before the
// primary telemetry persistence layer. That ordering preserves the incoming
// evidence while ensuring dropped dangerous tags do not reach the normal
// downstream representation.
func (processor *PolicyProcessor) Process(ctx context.Context, event domain.Event) (domain.Event, error) {
	if processor == nil || processor.engine == nil {
		return event, nil
	}
	processed, err := processor.engine.Evaluate(ctx, event)
	if err != nil {
		// Policy documents are validated at startup, so runtime failures are
		// currently persistence/dependency failures while recording findings,
		// diffs, or quarantine evidence. Treat them as transient and let the
		// worker's bounded retry policy handle a short database interruption.
		return domain.Event{}, reliability.New(reliability.Transient, "policy evaluation", err)
	}
	return processed, nil
}
