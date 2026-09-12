package wal

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
)

func TestAppendRecoverCommitAndSourceSequence(t *testing.T) {
	directory := t.TempDir()
	config := Config{Directory: directory, EdgeID: "edge-a", SegmentSizeBytes: 2048, MaxBytes: 8192}
	store, err := Open(config)
	if err != nil {
		t.Fatal(err)
	}
	first, _ := store.Append("telemetry.raw", testEvent("evt-1", "orders"))
	second, _ := store.Append("telemetry.raw", testEvent("evt-2", "orders"))
	third, _ := store.Append("telemetry.raw", testEvent("evt-3", "payments"))
	if first.EdgeSequence != 1 || second.EdgeSequence != 2 || third.EdgeSequence != 3 {
		t.Fatal("edge sequence mismatch")
	}
	if first.SourceSequence != 1 || second.SourceSequence != 2 || third.SourceSequence != 1 {
		t.Fatal("source sequence mismatch")
	}
	if err := store.Commit(1); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	recovered, err := Open(config)
	if err != nil {
		t.Fatal(err)
	}
	defer recovered.Close()
	pending, err := recovered.Pending(0)
	if err != nil || len(pending) != 2 {
		t.Fatalf("pending=%d err=%v", len(pending), err)
	}
	fourth, err := recovered.Append("telemetry.raw", testEvent("evt-4", "orders"))
	if err != nil {
		t.Fatal(err)
	}
	if fourth.EdgeSequence != 4 || fourth.SourceSequence != 3 {
		t.Fatalf("edge=%d source=%d", fourth.EdgeSequence, fourth.SourceSequence)
	}
}

func TestRecoverTruncatesOnlyIncompleteTail(t *testing.T) {
	directory := t.TempDir()
	config := Config{Directory: directory, EdgeID: "edge-a", SegmentSizeBytes: 4096, MaxBytes: 8192}
	store, err := Open(config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append("telemetry.raw", testEvent("evt-1", "orders")); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	segments, _ := filepath.Glob(filepath.Join(directory, "*.tfwal"))
	file, err := os.OpenFile(segments[0], os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = file.Write([]byte{0, 0, 1})
	_ = file.Close()
	recovered, err := Open(config)
	if err != nil {
		t.Fatal(err)
	}
	defer recovered.Close()
	pending, err := recovered.Pending(0)
	if err != nil || len(pending) != 1 || pending[0].Event.ID != "evt-1" {
		t.Fatalf("recovery failed: %#v %v", pending, err)
	}
}

func TestCorruptionFailsClosed(t *testing.T) {
	directory := t.TempDir()
	config := Config{Directory: directory, EdgeID: "edge-a", SegmentSizeBytes: 4096, MaxBytes: 8192}
	store, _ := Open(config)
	_, _ = store.Append("telemetry.raw", testEvent("evt-1", "orders"))
	_ = store.Close()
	segments, _ := filepath.Glob(filepath.Join(directory, "*.tfwal"))
	payload, _ := os.ReadFile(segments[0])
	payload[len(payload)-1] ^= 1
	_ = os.WriteFile(segments[0], payload, 0o640)
	if _, err := Open(config); err == nil {
		t.Fatal("expected corruption error")
	}
}

func TestDiskPressureRejectsBeforeAcceptance(t *testing.T) {
	store, err := Open(Config{Directory: t.TempDir(), EdgeID: "edge-a", SegmentSizeBytes: 512, MaxBytes: 512})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	accepted := 0
	for i := 0; i < 100; i++ {
		_, err := store.Append("telemetry.raw", testEvent("evt-pressure", "orders"))
		if errors.Is(err, ErrDiskPressure) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		accepted++
	}
	if accepted == 0 {
		t.Fatal("no records accepted")
	}
	if !errors.Is(store.Ready(), ErrDiskPressure) {
		t.Fatal("expected readiness backpressure")
	}
}

func TestCommittedSourceSequenceSurvivesCompactionAndRestart(t *testing.T) {
	directory := t.TempDir()
	config := Config{Directory: directory, EdgeID: "edge-a", SegmentSizeBytes: 700, MaxBytes: 4096}
	store, err := Open(config)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 6; i++ {
		record, err := store.Append("telemetry.raw", testEvent("evt", "orders"))
		if err != nil {
			t.Fatal(err)
		}
		if err := store.Commit(record.EdgeSequence); err != nil {
			t.Fatal(err)
		}
	}
	_ = store.Close()
	recovered, err := Open(config)
	if err != nil {
		t.Fatal(err)
	}
	defer recovered.Close()
	next, err := recovered.Append("telemetry.raw", testEvent("evt-next", "orders"))
	if err != nil {
		t.Fatal(err)
	}
	if next.SourceSequence != 7 {
		t.Fatalf("source sequence reset: %d", next.SourceSequence)
	}
}

func testEvent(id, source string) domain.Event {
	return domain.Event{ID: id, TenantID: "tenant-a", Source: source, Type: "test.event", Timestamp: time.Unix(1700000000, 0).UTC(), SchemaVersion: "1.0"}
}
