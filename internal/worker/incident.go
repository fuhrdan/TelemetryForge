package worker

import (
	"context"
	"log/slog"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
)

// IncidentObserver evaluates a successfully persisted event for incident rules.
type IncidentObserver interface {
	Observe(ctx context.Context, event domain.Event) error
}

// IncidentDetector adapts an incident observer into the worker processor chain.
type IncidentDetector struct {
	observer IncidentObserver
	logger   *slog.Logger
}

// NewIncidentDetector creates a worker processing stage.
func NewIncidentDetector(observer IncidentObserver, logger *slog.Logger) *IncidentDetector {
	return &IncidentDetector{observer: observer, logger: logger}
}

// Process evaluates the event and passes it through unchanged.
//
// This processor belongs after persistence in the chain. If incident capture
// fails, the primary event is still valid and already durable. Retrying the
// whole chain would create extra Flight Recorder writes and incorrectly treat
// an operational side-feature failure as telemetry loss.
func (detector *IncidentDetector) Process(ctx context.Context, event domain.Event) (domain.Event, error) {
	if detector == nil || detector.observer == nil {
		return event, nil
	}
	if err := detector.observer.Observe(ctx, event); err != nil {
		if detector.logger != nil {
			detector.logger.Error(
				"automatic incident capture failed",
				"event_id", event.ID,
				"source", event.Source,
				"error", err,
			)
		}
	}
	return event, nil
}
