package worker

import (
	"context"
	"errors"
	"testing"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
)

type memoryFlightWriter struct {
	event domain.Event
	err   error
}

func (writer *memoryFlightWriter) WriteFlightEvent(_ context.Context, event domain.Event) error {
	writer.event = event
	return writer.err
}

func TestFlightRecorderPreservesIncomingEnvelope(t *testing.T) {
	writer := &memoryFlightWriter{}
	recorder, err := NewFlightRecorder(writer)
	if err != nil {
		t.Fatal(err)
	}

	input := domain.Event{ID: "evt-1", Source: " checkout ", Type: " request.duration "}
	output, err := recorder.Process(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if writer.event.Source != " checkout " || output.Source != input.Source {
		t.Fatal("flight recorder must preserve the incoming pre-normalized envelope")
	}
}

func TestFlightRecorderTreatsStorageFailureAsTransient(t *testing.T) {
	writer := &memoryFlightWriter{err: errors.New("database down")}
	recorder, _ := NewFlightRecorder(writer)
	if _, err := recorder.Process(context.Background(), domain.Event{ID: "evt"}); err == nil {
		t.Fatal("expected recorder failure")
	}
}
