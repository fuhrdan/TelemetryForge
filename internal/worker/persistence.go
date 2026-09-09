package worker

import (
	"context"
	"errors"
	"fmt"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
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
		return domain.Event{}, fmt.Errorf("persist event: %w", err)
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
	var err error
	for _, processor := range chain.processors {
		event, err = processor.Process(ctx, event)
		if err != nil {
			return domain.Event{}, err
		}
	}
	return event, nil
}
