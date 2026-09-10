package worker

import (
	"context"
	"testing"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/changeintel"
	"github.com/fuhrdan/TelemetryForge/internal/domain"
)

type memoryChangeWriter struct {
	markers []changeintel.Marker
	err     error
}

func (w *memoryChangeWriter) RecordChangeMarker(_ context.Context, m changeintel.Marker) error {
	if w.err != nil {
		return w.err
	}
	w.markers = append(w.markers, m)
	return nil
}

func TestChangeRecorderPersistsOnlyChangeEvents(t *testing.T) {
	writer := &memoryChangeWriter{}
	recorder, _ := NewChangeRecorder(writer)
	normal := domain.Event{ID: "n", Source: "api", Type: "request", Timestamp: time.Now(), SchemaVersion: "1"}
	if _, err := recorder.Process(context.Background(), normal); err != nil {
		t.Fatal(err)
	}
	if len(writer.markers) != 0 {
		t.Fatal("ordinary event should not create change marker")
	}
	change := domain.Event{ID: "d", TenantID: "alpha", Source: "api", Type: "deployment.completed", Timestamp: time.Now(), SchemaVersion: "1", Tags: map[string]string{"deployment_id": "dep1", "version": "2"}}
	if _, err := recorder.Process(context.Background(), change); err != nil {
		t.Fatal(err)
	}
	if len(writer.markers) != 1 || writer.markers[0].ChangeID != "dep1" {
		t.Fatalf("markers=%#v", writer.markers)
	}
}
