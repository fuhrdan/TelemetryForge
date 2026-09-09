package worker

import (
	"context"
	"errors"
	"testing"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
)

type memoryWriter struct {
	count int
	event domain.Event
	err   error
}

func (writer *memoryWriter) WriteEvent(_ context.Context, event domain.Event) error {
	writer.count++
	writer.event = event
	return writer.err
}

func TestPersisterWritesEvent(t *testing.T) {
	writer := &memoryWriter{}
	persister, err := NewPersister(writer)
	if err != nil {
		t.Fatal(err)
	}

	event := domain.Event{ID: "evt-1", Source: "checkout", Type: "request"}
	processed, err := persister.Process(context.Background(), event)
	if err != nil {
		t.Fatal(err)
	}
	if writer.count != 1 || processed.ID != event.ID {
		t.Fatalf("event was not persisted correctly")
	}
}

func TestPersisterPropagatesStorageFailure(t *testing.T) {
	writer := &memoryWriter{err: errors.New("database unavailable")}
	persister, _ := NewPersister(writer)

	_, err := persister.Process(context.Background(), domain.Event{ID: "evt-1"})
	if err == nil {
		t.Fatal("expected persistence failure")
	}
}

func TestChainNormalizesBeforePersistence(t *testing.T) {
	writer := &memoryWriter{}
	persister, _ := NewPersister(writer)
	chain, err := NewChain(Normalizer{}, persister)
	if err != nil {
		t.Fatal(err)
	}

	_, err = chain.Process(context.Background(), domain.Event{
		ID: "evt-1", Source: " checkout ", Type: " request.duration ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if writer.event.Source != "checkout" || writer.event.Type != "request.duration" {
		t.Fatalf("stored event was not normalized: %#v", writer.event)
	}
}
