package edge

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/replication"
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

func TestPublisherRequiresReplicationBeforeSuccessfulAcceptance(t *testing.T) {
	peerNode := replication.Node{ID: "edge-b", Domain: replication.FailureDomain{Cloud: "aws", Region: "us-west-2", Zone: "b"}}
	peerStore, err := replication.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer peerStore.Close()
	peerServer := httptest.NewServer(replication.NewHandler(peerStore, peerNode, "secret"))
	defer peerServer.Close()

	manager, err := replication.NewManager(replication.Config{
		Local:   replication.Node{ID: "edge-a", Domain: replication.FailureDomain{Cloud: "aws", Region: "us-west-2", Zone: "a"}},
		Peers:   []replication.Node{{ID: peerNode.ID, URL: peerServer.URL, Domain: peerNode.Domain}},
		Mode:    replication.ModeRegional,
		Quorum:  2,
		Timeout: time.Second,
		Token:   "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	store, err := wal.Open(wal.Config{Directory: t.TempDir(), EdgeID: "edge-a", SegmentSizeBytes: 4096, MaxBytes: 8192})
	if err != nil {
		t.Fatal(err)
	}
	downstream := stream.NewMemoryPublisher()
	publisher := NewPublisher(store, downstream, slog.New(slog.NewTextHandler(io.Discard, nil)), manager)
	defer publisher.Close()
	event := domain.Event{ID: "evt-replicated", TenantID: "tenant-a", Source: "orders", Type: "test.event", Timestamp: time.Now().UTC(), SchemaVersion: "1.0"}
	if err := publisher.Publish(context.Background(), "telemetry.raw", event); err != nil {
		t.Fatal(err)
	}
	status := manager.Status()
	if !status.LastSatisfied || status.LastAckCount < 2 {
		t.Fatalf("successful acceptance did not record replicated quorum: %#v", status)
	}
}

func TestPublisherQuorumFailureLeavesLocalRecordPending(t *testing.T) {
	peerNode := replication.Node{ID: "edge-b", Domain: replication.FailureDomain{Cloud: "aws", Region: "us-west-2", Zone: "b"}}
	peerStore, _ := replication.OpenStore(t.TempDir())
	peerServer := httptest.NewServer(replication.NewHandler(peerStore, peerNode, "secret"))
	peerURL := peerServer.URL
	peerServer.Close()
	defer peerStore.Close()

	manager, err := replication.NewManager(replication.Config{
		Local:   replication.Node{ID: "edge-a", Domain: replication.FailureDomain{Cloud: "aws", Region: "us-west-2", Zone: "a"}},
		Peers:   []replication.Node{{ID: peerNode.ID, URL: peerURL, Domain: peerNode.Domain}},
		Mode:    replication.ModeRegional,
		Quorum:  2,
		Timeout: 100 * time.Millisecond,
		Token:   "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	store, err := wal.Open(wal.Config{Directory: t.TempDir(), EdgeID: "edge-a", SegmentSizeBytes: 4096, MaxBytes: 8192})
	if err != nil {
		t.Fatal(err)
	}
	downstream := stream.NewMemoryPublisher()
	publisher := NewPublisher(store, downstream, slog.New(slog.NewTextHandler(io.Discard, nil)), manager)
	defer publisher.Close()
	event := domain.Event{ID: "evt-no-quorum", TenantID: "tenant-a", Source: "orders", Type: "test.event", Timestamp: time.Now().UTC(), SchemaVersion: "1.0"}
	err = publisher.Publish(context.Background(), "telemetry.raw", event)
	if !errors.Is(err, replication.ErrQuorumUnavailable) {
		t.Fatalf("expected quorum error, got %v", err)
	}
	if publisher.Stats().PendingRecords != 1 {
		t.Fatal("failed quorum record must remain in local WAL")
	}
	time.Sleep(150 * time.Millisecond)
	if len(downstream.Events()) != 0 {
		t.Fatal("record reached downstream without replication quorum")
	}
}
