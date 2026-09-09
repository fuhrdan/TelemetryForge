package worker

import (
	"context"
	"errors"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/reliability"
)

// FlightWriter stores a short-lived full-fidelity copy of telemetry.
type FlightWriter interface {
	WriteFlightEvent(ctx context.Context, event domain.Event) error
}

// FlightRecorder is the first implementation primitive for TelemetryForge's
// incident "black box" feature.
type FlightRecorder struct {
	writer FlightWriter
}

// NewFlightRecorder creates a recording processor.
func NewFlightRecorder(writer FlightWriter) (*FlightRecorder, error) {
	if writer == nil {
		return nil, errors.New("flight-recorder writer is required")
	}
	return &FlightRecorder{writer: writer}, nil
}

// Process records the pre-normalized event then passes it through unchanged.
func (recorder *FlightRecorder) Process(ctx context.Context, event domain.Event) (domain.Event, error) {
	if err := recorder.writer.WriteFlightEvent(ctx, event); err != nil {
		return domain.Event{}, reliability.New(reliability.Transient, "flight recorder", err)
	}
	return event, nil
}
