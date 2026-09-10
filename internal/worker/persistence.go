package worker

import (
	"context"
	"errors"
	"fmt"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/reliability"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// EventWriter is the storage behavior required by the processing layer.
type EventWriter interface {
	WriteEvent(ctx context.Context, event domain.Event) error
}

// Persister writes a processed event to durable storage.
type Persister struct {
	writer EventWriter
}

// NewPersister creates a persistence processor.
func NewPersister(writer EventWriter) (*Persister, error) {
	if writer == nil {
		return nil, errors.New("event writer is required")
	}
	return &Persister{writer: writer}, nil
}

// Process durably stores the event and returns it unchanged.
//
// Kafka acknowledgement happens after this method succeeds. Database failure
// therefore leaves the Kafka record uncommitted for later retry.
func (persister *Persister) Process(ctx context.Context, event domain.Event) (domain.Event, error) {
	if err := persister.writer.WriteEvent(ctx, event); err != nil {
		return domain.Event{}, reliability.New(reliability.Transient, "persist event", err)
	}
	return event, nil
}

// Chain runs processors in order.
//
// Normalization occurs before persistence so the stored representation is the
// same canonical representation later query and replay features will see.
type Chain struct {
	processors []Processor
}

// NewChain creates an ordered processing pipeline.
func NewChain(processors ...Processor) (*Chain, error) {
	if len(processors) == 0 {
		return nil, errors.New("at least one processor is required")
	}
	for _, processor := range processors {
		if processor == nil {
			return nil, errors.New("processor chain cannot contain nil")
		}
	}
	return &Chain{processors: processors}, nil
}

// Process runs the event through every configured processor.
func (chain *Chain) Process(ctx context.Context, event domain.Event) (domain.Event, error) {
	tracer := otel.Tracer("github.com/fuhrdan/TelemetryForge/worker/pipeline")

	for _, processor := range chain.processors {
		processorName := fmt.Sprintf("%T", processor)
		stageCtx, span := tracer.Start(ctx, "worker.stage")
		span.SetAttributes(attribute.String("telemetryforge.processor", processorName))

		processed, err := processor.Process(stageCtx, event)
		if err != nil {
			if isStopProcessing(err) {
				span.SetAttributes(attribute.String("telemetryforge.pipeline_control", "sampled_out"))
				span.End()
				return processed, nil
			}
			span.RecordError(err)
			span.SetStatus(codes.Error, "processor failed")
			span.End()
			return domain.Event{}, err
		}
		span.End()
		event = processed
	}
	return event, nil
}
