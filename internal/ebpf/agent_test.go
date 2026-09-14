package ebpf

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
)

type fakeBackend struct {
	mu        sync.Mutex
	snapshots []Snapshot
	index     int
	startErr  error
}

func (backend *fakeBackend) Start() error { return backend.startErr }
func (backend *fakeBackend) Close() error { return nil }
func (backend *fakeBackend) Name() string { return "fake-ebpf" }
func (backend *fakeBackend) Probes() []Probe {
	return []Probe{{Signal: SignalProcessExec, Group: "sched", Event: "sched_process_exec", Active: backend.startErr == nil}}
}
func (backend *fakeBackend) Snapshot() (Snapshot, error) {
	backend.mu.Lock()
	defer backend.mu.Unlock()
	if len(backend.snapshots) == 0 {
		return Snapshot{}, errors.New("no snapshots")
	}
	if backend.index >= len(backend.snapshots) {
		return backend.snapshots[len(backend.snapshots)-1], nil
	}
	out := backend.snapshots[backend.index]
	backend.index++
	return out, nil
}

type publishResult struct {
	persisted bool
	err       error
}

type fakePublisher struct {
	mu      sync.Mutex
	results []publishResult
	events  []domain.Event
}

func (publisher *fakePublisher) PublishKernelMetric(_ context.Context, _ string, event domain.Event) (bool, error) {
	publisher.mu.Lock()
	defer publisher.mu.Unlock()
	publisher.events = append(publisher.events, event)
	if len(publisher.results) == 0 {
		return true, nil
	}
	result := publisher.results[0]
	publisher.results = publisher.results[1:]
	return result.persisted, result.err
}

func TestParseCPUList(t *testing.T) {
	cpus, err := parseCPUList("0-3,8,10-11")
	if err != nil {
		t.Fatal(err)
	}
	want := []int{0, 1, 2, 3, 8, 10, 11}
	if len(cpus) != len(want) {
		t.Fatalf("cpus=%v", cpus)
	}
	for index := range want {
		if cpus[index] != want[index] {
			t.Fatalf("cpus=%v want=%v", cpus, want)
		}
	}
}

func TestAgentAdvancesAfterLocalPersistenceEvenWhenReplicationFails(t *testing.T) {
	at := time.Unix(1_700_000_000, 0).UTC()
	backend := &fakeBackend{snapshots: []Snapshot{
		{Totals: map[string]uint64{SignalProcessExec: 5}, At: at},
		{Totals: map[string]uint64{SignalProcessExec: 5}, At: at.Add(time.Second)},
	}}
	publisher := &fakePublisher{results: []publishResult{{persisted: true, err: errors.New("replication quorum unavailable")}}}
	agent, err := NewAgent(Config{Enabled: true, PollInterval: time.Hour, Topic: "metrics", Source: "edge-kernel", EdgeID: "edge-a"}, backend, publisher)
	if err != nil {
		t.Fatal(err)
	}
	agent.poll(context.Background())
	agent.poll(context.Background())
	if got := len(publisher.events); got != 1 {
		t.Fatalf("expected one publish attempt after durable local persistence, got %d", got)
	}
	if got := agent.Status().PersistedTotals[SignalProcessExec]; got != 5 {
		t.Fatalf("persisted total=%d want=5", got)
	}
}

func TestAgentRetriesCounterDeltaWhenLocalPersistenceFails(t *testing.T) {
	at := time.Unix(1_700_000_000, 0).UTC()
	backend := &fakeBackend{snapshots: []Snapshot{
		{Totals: map[string]uint64{SignalSocketConnect: 3}, At: at},
		{Totals: map[string]uint64{SignalSocketConnect: 3}, At: at.Add(time.Second)},
	}}
	publisher := &fakePublisher{results: []publishResult{{persisted: false, err: errors.New("wal full")}, {persisted: true}}}
	agent, err := NewAgent(Config{Enabled: true, PollInterval: time.Hour, Topic: "metrics", EdgeID: "edge-a"}, backend, publisher)
	if err != nil {
		t.Fatal(err)
	}
	agent.poll(context.Background())
	agent.poll(context.Background())
	if got := len(publisher.events); got != 2 {
		t.Fatalf("expected delta retry, got %d publish attempts", got)
	}
	if publisher.events[0].ID != publisher.events[1].ID {
		t.Fatalf("retry event id changed: %q != %q", publisher.events[0].ID, publisher.events[1].ID)
	}
	if got := agent.Status().PersistedTotals[SignalSocketConnect]; got != 3 {
		t.Fatalf("persisted total=%d want=3", got)
	}
}

func TestKernelMetricEnvelopeContainsNoPayload(t *testing.T) {
	at := time.Unix(1_700_000_000, 0).UTC()
	backend := &fakeBackend{snapshots: []Snapshot{{Totals: map[string]uint64{SignalTCPRetransmit: 2}, At: at}}}
	publisher := &fakePublisher{}
	agent, _ := NewAgent(Config{Enabled: true, PollInterval: time.Hour, Topic: "metrics", Source: "kernel", TenantID: "system", EdgeID: "edge-a"}, backend, publisher)
	agent.poll(context.Background())
	if len(publisher.events) != 1 {
		t.Fatalf("events=%d", len(publisher.events))
	}
	event := publisher.events[0]
	if len(event.Payload) != 0 {
		t.Fatalf("kernel metric unexpectedly contains payload: %s", event.Payload)
	}
	if event.TenantID != "system" {
		t.Fatalf("tenant=%q want system", event.TenantID)
	}
	if event.Type != MetricName(SignalTCPRetransmit) {
		t.Fatalf("type=%q", event.Type)
	}
	if event.Value == nil || *event.Value != 2 {
		t.Fatalf("value=%v", event.Value)
	}
	if event.Tags["collector"] != "ebpf" || event.Tags["signal"] != SignalTCPRetransmit {
		t.Fatalf("tags=%v", event.Tags)
	}
}

func TestOptionalStartFailureDoesNotFailAgent(t *testing.T) {
	backend := &fakeBackend{startErr: errors.New("operation not permitted")}
	publisher := &fakePublisher{}
	agent, _ := NewAgent(Config{Enabled: true, Required: false, PollInterval: time.Millisecond, Topic: "metrics"}, backend, publisher)
	if err := agent.Run(context.Background()); err != nil {
		t.Fatalf("optional collector returned error: %v", err)
	}
	if agent.Status().LastError == "" {
		t.Fatal("expected degraded status error")
	}
}

func TestRequiredStartFailureFailsAgent(t *testing.T) {
	backend := &fakeBackend{startErr: errors.New("operation not permitted")}
	publisher := &fakePublisher{}
	agent, _ := NewAgent(Config{Enabled: true, Required: true, PollInterval: time.Millisecond, Topic: "metrics"}, backend, publisher)
	if err := agent.Run(context.Background()); err == nil {
		t.Fatal("expected required collector start failure")
	}
}
