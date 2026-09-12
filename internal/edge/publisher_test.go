package edge

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/stream"
	"github.com/fuhrdan/TelemetryForge/internal/wal"
)

func TestPublisherAcceptsDuringDownstreamOutageAndReplays(t *testing.T) {
	store, err := wal.Open(wal.Config{Directory: t.TempDir(), EdgeID: "edge-test", SegmentSizeBytes: 4096, MaxBytes: 8192})
	if err != nil {
		t.Fatal(err)
	}
	downstream := stream.NewMemoryPublisher()
	downstream.SetError(errors.New("downstream unavailable"))
	publisher := NewPublisher(store, downstream, slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer publisher.Close()
	event := domain.Event{ID: "evt-1", TenantID: "tenant-a", Source: "orders", Type: "test.event", Timestamp: time.Now().UTC(), SchemaVersion: "1.0"}
	if err := publisher.Publish(context.Background(), "telemetry.raw", event); err != nil {
		t.Fatal(err)
	}
	if publisher.Stats().PendingRecords != 1 {
		t.Fatal("expected durable pending record")
	}
	downstream.SetError(nil)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if len(downstream.Events()) == 1 && publisher.Stats().PendingRecords == 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("record was not replayed after downstream recovery")
}
