package replication

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/wal"
)

func TestRegionalPolicyRequiresSecondZoneInLocalRegion(t *testing.T) {
	config := normalizedConfig(Config{
		Local:  Node{ID: "edge-a", Domain: FailureDomain{Cloud: "aws", Region: "us-west-2", Zone: "a"}},
		Mode:   ModeRegional,
		Quorum: 2,
	})
	result := evaluate(config, []Ack{
		{Node: config.Local},
		{Node: Node{ID: "edge-b", Domain: FailureDomain{Cloud: "aws", Region: "us-west-2", Zone: "b"}}},
	})
	if !result.Satisfied || result.AckCount != 2 {
		t.Fatalf("expected regional quorum, got %#v", result)
	}
	result = evaluate(config, []Ack{
		{Node: config.Local},
		{Node: Node{ID: "edge-c", Domain: FailureDomain{Cloud: "gcp", Region: "us-central1", Zone: "c"}}},
	})
	if result.Satisfied || result.AckCount != 1 {
		t.Fatalf("other region must not satisfy regional quorum: %#v", result)
	}
}

func TestCrossRegionAndCrossCloudPolicies(t *testing.T) {
	local := Node{ID: "edge-a", Domain: FailureDomain{Cloud: "aws", Region: "us-west-2", Zone: "a"}}
	peerRegion := Node{ID: "edge-b", Domain: FailureDomain{Cloud: "aws", Region: "us-east-1", Zone: "b"}}
	peerCloud := Node{ID: "edge-c", Domain: FailureDomain{Cloud: "gcp", Region: "us-central1", Zone: "c"}}
	if !evaluate(normalizedConfig(Config{Local: local, Mode: ModeCrossRegion, Quorum: 2}), []Ack{{Node: local}, {Node: peerRegion}}).Satisfied {
		t.Fatal("expected cross-region quorum")
	}
	if evaluate(normalizedConfig(Config{Local: local, Mode: ModeCrossCloud, Quorum: 2}), []Ack{{Node: local}, {Node: peerRegion}}).Satisfied {
		t.Fatal("same-cloud peer must not satisfy cross-cloud quorum")
	}
	if !evaluate(normalizedConfig(Config{Local: local, Mode: ModeCrossCloud, Quorum: 2}), []Ack{{Node: local}, {Node: peerCloud}}).Satisfied {
		t.Fatal("expected cross-cloud quorum")
	}
}

func TestReplicaStoreIsIdempotentAndRecovers(t *testing.T) {
	directory := t.TempDir()
	record := testRecord(t, "edge-origin", 1, "evt-1")
	store, err := OpenStore(directory)
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := store.Put(record)
	if err != nil || duplicate {
		t.Fatalf("first put duplicate=%v err=%v", duplicate, err)
	}
	duplicate, err = store.Put(record)
	if err != nil || !duplicate {
		t.Fatalf("second put duplicate=%v err=%v", duplicate, err)
	}
	if store.Count("edge-origin") != 1 {
		t.Fatal("duplicate changed replica count")
	}
	_ = store.Close()

	recovered, err := OpenStore(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer recovered.Close()
	if recovered.Count("edge-origin") != 1 {
		t.Fatal("replica was not recovered")
	}

	conflict := testRecord(t, "edge-origin", 1, "evt-conflict")
	if _, err := recovered.Put(conflict); !errors.Is(err, ErrReplicaConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestReplicaRecoveryTruncatesIncompleteTail(t *testing.T) {
	directory := t.TempDir()
	store, err := OpenStore(directory)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Put(testRecord(t, "edge-origin", 1, "evt-1")); err != nil {
		t.Fatal(err)
	}
	_ = store.Close()
	paths, err := filepath.Glob(filepath.Join(directory, "*.tfreplica"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("paths=%v err=%v", paths, err)
	}
	file, err := os.OpenFile(paths[0], os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = file.Write([]byte{0, 0, 0})
	_ = file.Close()
	recovered, err := OpenStore(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer recovered.Close()
	if recovered.Count("edge-origin") != 1 {
		t.Fatal("valid replica lost during tail recovery")
	}
}

func TestManagerReplicatesToRegionalQuorum(t *testing.T) {
	token := "test-secret"
	peerNode := Node{ID: "edge-b", Domain: FailureDomain{Cloud: "aws", Region: "us-west-2", Zone: "b"}}
	peerStore, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer peerStore.Close()
	server := httptest.NewServer(NewHandler(peerStore, peerNode, token))
	defer server.Close()

	local := Node{ID: "edge-a", Domain: FailureDomain{Cloud: "aws", Region: "us-west-2", Zone: "a"}}
	manager, err := NewManager(Config{Local: local, Peers: []Node{{ID: peerNode.ID, URL: server.URL, Domain: peerNode.Domain}}, Mode: ModeRegional, Quorum: 2, Timeout: time.Second, Token: token})
	if err != nil {
		t.Fatal(err)
	}
	record := testRecord(t, local.ID, 1, "evt-1")
	result, err := manager.Replicate(context.Background(), record)
	if err != nil || !result.Satisfied || result.AckCount != 2 {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if peerStore.Count(local.ID) != 1 {
		t.Fatal("peer did not durably persist record")
	}
	if err := manager.Ready(context.Background()); err != nil {
		t.Fatalf("manager readiness failed: %v", err)
	}
}

func TestHandlerRejectsMissingToken(t *testing.T) {
	store, _ := OpenStore(t.TempDir())
	defer store.Close()
	handler := NewHandler(store, Node{ID: "edge-b"}, "secret")
	request := httptest.NewRequest(http.MethodGet, "/internal/v1/replication/health", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", response.Code)
	}
}

func TestParsePeers(t *testing.T) {
	peers, err := ParsePeers("edge-b|http://edge-b:8083|aws|us-west-2|b,edge-c|https://edge-c.example|gcp|us-central1|c")
	if err != nil || len(peers) != 2 {
		t.Fatalf("peers=%#v err=%v", peers, err)
	}
	if peers[1].Domain.Cloud != "gcp" || peers[1].URL != "https://edge-c.example" {
		t.Fatalf("unexpected peer: %#v", peers[1])
	}
}

func testRecord(t *testing.T, edgeID string, sequence uint64, eventID string) wal.Record {
	t.Helper()
	event := domain.Event{ID: eventID, TenantID: "tenant-a", Source: "orders", Type: "test.event", Timestamp: time.Unix(1700000000, 0).UTC(), SchemaVersion: "1.0"}
	hash, err := wal.EventHash(event)
	if err != nil {
		t.Fatal(err)
	}
	record := wal.Record{Format: wal.Format, FormatVersion: wal.Version, EdgeID: edgeID, Topic: "telemetry.raw", EdgeSequence: sequence, SourceSequence: sequence, AcceptedAt: time.Unix(1700000001, 0).UTC(), PayloadSHA256: hash, LineageVersion: wal.LineageVersion, Event: event}
	if sequence > 1 {
		record.PreviousRecordHash = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	}
	record.RecordHash, err = wal.ComputeRecordHash(record)
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func TestAckJSONShape(t *testing.T) {
	payload, err := json.Marshal(Ack{Node: Node{ID: "edge-b"}, EdgeID: "edge-a", EdgeSequence: 7})
	if err != nil || len(payload) == 0 {
		t.Fatal("ack must remain JSON serializable")
	}
}

func TestReplicaReleaseMakesOldSequenceIdempotent(t *testing.T) {
	directory := t.TempDir()
	store, err := OpenStoreWithConfig(StoreConfig{Directory: directory, MaxBytes: 4096})
	if err != nil {
		t.Fatal(err)
	}
	record := testRecord(t, "edge-origin", 1, "evt-1")
	if _, err := store.Put(record); err != nil {
		t.Fatal(err)
	}
	if err := store.Release("edge-origin", 1); err != nil {
		t.Fatal(err)
	}
	if store.Count("edge-origin") != 0 {
		t.Fatal("released record remained retained")
	}
	duplicate, err := store.Put(record)
	if err != nil || !duplicate {
		t.Fatalf("released retry duplicate=%v err=%v", duplicate, err)
	}
	_ = store.Close()

	recovered, err := OpenStoreWithConfig(StoreConfig{Directory: directory, MaxBytes: 4096})
	if err != nil {
		t.Fatal(err)
	}
	defer recovered.Close()
	duplicate, err = recovered.Put(record)
	if err != nil || !duplicate {
		t.Fatalf("release checkpoint did not survive restart: duplicate=%v err=%v", duplicate, err)
	}
}

func TestReplicaPressureCompactsReleasedRecords(t *testing.T) {
	directory := t.TempDir()
	probe := testRecord(t, "edge-origin", 1, "evt-1")
	_, payload, err := fingerprintRecord(probe)
	if err != nil {
		t.Fatal(err)
	}
	second := testRecord(t, "edge-origin", 2, "evt-2")
	_, secondPayload, err := fingerprintRecord(second)
	if err != nil {
		t.Fatal(err)
	}
	largest := len(payload)
	if len(secondPayload) > largest {
		largest = len(secondPayload)
	}
	maxBytes := int64(len(replicaHeader) + replicaFrameHeaderBytes + largest + 32)
	store, err := OpenStoreWithConfig(StoreConfig{Directory: directory, MaxBytes: maxBytes})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.Put(probe); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Put(second); !errors.Is(err, ErrReplicaPressure) {
		t.Fatalf("expected replica pressure, got %v", err)
	}
	if err := store.Release("edge-origin", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Put(second); err != nil {
		t.Fatalf("released space was not reclaimed on pressure: %v", err)
	}
}

func TestReleaseEndpointPersistsCheckpoint(t *testing.T) {
	store, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	record := testRecord(t, "edge-origin", 1, "evt-1")
	if _, err := store.Put(record); err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(store, Node{ID: "edge-b"}, "secret")
	body := strings.NewReader(`{"origin_edge_id":"edge-origin","released_through":1}`)
	request := httptest.NewRequest(http.MethodPost, "/internal/v1/replication/release", body)
	request.Header.Set("Authorization", "Bearer secret")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", response.Code, response.Body.String())
	}
	if store.Count("edge-origin") != 0 {
		t.Fatal("release endpoint did not release retained replica")
	}
}
