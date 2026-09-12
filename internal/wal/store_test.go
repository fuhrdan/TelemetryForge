package wal

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"hash/crc32"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/lineage"
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
	store, err := Open(Config{Directory: t.TempDir(), EdgeID: "edge-a", SegmentSizeBytes: 1024, MaxBytes: 1024})
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

func TestLineageHashChainAndSignedSeal(t *testing.T) {
	directory := t.TempDir()
	store, err := Open(Config{Directory: directory, EdgeID: "edge-a", SegmentSizeBytes: 1200, MaxBytes: 8192})
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.Append("telemetry.raw", testEvent("evt-1", "orders"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Append("telemetry.raw", testEvent("evt-2", "orders"))
	if err != nil {
		t.Fatal(err)
	}
	if second.PreviousRecordHash != first.RecordHash {
		t.Fatalf("record chain mismatch: got %s want %s", second.PreviousRecordHash, first.RecordHash)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	seals, err := filepath.Glob(filepath.Join(directory, "*.tfseal"))
	if err != nil || len(seals) != 1 {
		t.Fatalf("expected one signed seal, seals=%v err=%v", seals, err)
	}
	report, err := AuditDirectory(directory, filepath.Join(directory, defaultPublicKey))
	if err != nil {
		t.Fatal(err)
	}
	if report.SealedSegments != 1 || report.ContentSegments != 1 || report.LastRecordHash != second.RecordHash {
		t.Fatalf("unexpected audit report: %+v", report)
	}
}

func TestAuditDetectsTamperedSeal(t *testing.T) {
	directory := t.TempDir()
	store, err := Open(Config{Directory: directory, EdgeID: "edge-a", SegmentSizeBytes: 4096, MaxBytes: 8192})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append("telemetry.raw", testEvent("evt-1", "orders")); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	seals, _ := filepath.Glob(filepath.Join(directory, "*.tfseal"))
	payload, err := os.ReadFile(seals[0])
	if err != nil {
		t.Fatal(err)
	}
	for i := range payload {
		if payload[i] == 'a' {
			payload[i] = 'b'
			break
		}
	}
	if err := os.WriteFile(seals[0], payload, 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := AuditDirectory(directory, ""); err == nil {
		t.Fatal("expected tampered lineage seal verification to fail")
	}
}

func TestLegacyWALUpgradeCreatesSignedAnchor(t *testing.T) {
	directory := t.TempDir()
	event := testEvent("legacy-1", "orders")
	hash, err := EventHash(event)
	if err != nil {
		t.Fatal(err)
	}
	record := Record{Format: Format, FormatVersion: LegacyVersion, EdgeID: "edge-a", Topic: "telemetry.raw", EdgeSequence: 1, SourceSequence: 1, AcceptedAt: time.Unix(1700000001, 0).UTC(), PayloadSHA256: hash, Event: event}
	payload, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "00000000000000000001.tfwal")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o640)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte(segmentHeader)); err != nil {
		t.Fatal(err)
	}
	var header [frameHeaderBytes]byte
	binary.BigEndian.PutUint32(header[0:4], uint32(len(payload)))
	binary.BigEndian.PutUint32(header[4:8], crc32.ChecksumIEEE(payload))
	if _, err := file.Write(header[:]); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	legacyDigest, err := RecordDigest(record)
	if err != nil {
		t.Fatal(err)
	}
	store, err := Open(Config{Directory: directory, EdgeID: "edge-a", SegmentSizeBytes: 4096, MaxBytes: 8192})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	seals, _ := filepath.Glob(filepath.Join(directory, "*.tfseal"))
	if len(seals) != 1 {
		t.Fatalf("expected upgrade seal, got %d", len(seals))
	}
	next, err := store.Append("telemetry.raw", testEvent("v2-2", "orders"))
	if err != nil {
		t.Fatal(err)
	}
	if next.FormatVersion != Version || next.PreviousRecordHash != legacyDigest {
		t.Fatalf("v2 lineage did not anchor legacy record: %+v", next)
	}
}

func TestCompactionRetainsSignedSeal(t *testing.T) {
	directory := t.TempDir()
	store, err := Open(Config{Directory: directory, EdgeID: "edge-a", SegmentSizeBytes: 900, MaxBytes: 8192})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for i := 0; i < 10; i++ {
		if _, err := store.Append("telemetry.raw", testEvent("evt-compact", "orders")); err != nil {
			t.Fatal(err)
		}
		segments, _ := filepath.Glob(filepath.Join(directory, "*.tfwal"))
		if len(segments) > 1 {
			break
		}
	}
	seals, err := filepath.Glob(filepath.Join(directory, "*.tfseal"))
	if err != nil || len(seals) == 0 {
		t.Fatalf("expected rotated signed segment: seals=%v err=%v", seals, err)
	}
	seal, err := lineage.ReadSeal(seals[0])
	if err != nil {
		t.Fatal(err)
	}
	for sequence := uint64(1); sequence <= seal.LastSequence; sequence++ {
		if err := store.Commit(sequence); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(directory, seal.Segment)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected compacted WAL payload to be removed, err=%v", err)
	}
	if _, err := os.Stat(seals[0]); err != nil {
		t.Fatalf("signed seal should survive compaction: %v", err)
	}
	report, err := AuditDirectory(directory, filepath.Join(directory, defaultPublicKey))
	if err != nil {
		t.Fatal(err)
	}
	if report.SealOnlySegments == 0 || !report.SegmentChainVerified || !report.RecordChainVerified {
		t.Fatalf("unexpected post-compaction audit report: %+v", report)
	}
}
