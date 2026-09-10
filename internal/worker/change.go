package worker

import (
	"context"
	"errors"

	"github.com/fuhrdan/TelemetryForge/internal/changeintel"
	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/reliability"
)

// ChangeWriter stores structured deployment/change evidence before lossy
// shaping. Canonical telemetry still proceeds through the normal pipeline.
type ChangeWriter interface {
	RecordChangeMarker(context.Context, changeintel.Marker) error
}

// ChangeRecorder recognizes change events and persists their normalized marker.
type ChangeRecorder struct{ writer ChangeWriter }

func NewChangeRecorder(writer ChangeWriter) (*ChangeRecorder, error) {
	if writer == nil {
		return nil, errors.New("change writer is required")
	}
	return &ChangeRecorder{writer: writer}, nil
}

func (recorder *ChangeRecorder) Process(ctx context.Context, event domain.Event) (domain.Event, error) {
	marker, ok := changeintel.MarkerFromEvent(event)
	if !ok {
		return event, nil
	}
	if err := recorder.writer.RecordChangeMarker(ctx, marker); err != nil {
		return domain.Event{}, reliability.New(reliability.Transient, "record change marker", err)
	}
	return event, nil
}
