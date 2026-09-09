package replay

import (
	"context"
	"testing"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/policy"
)

type memoryRepository struct {
	events  []domain.Event
	run     Run
	results []EventResult
}

func (repo *memoryRepository) IncidentEvents(context.Context, string, int) ([]domain.Event, error) {
	return repo.events, nil
}
func (repo *memoryRepository) StartReplay(_ context.Context, run Run) error {
	repo.run = run
	return nil
}
func (repo *memoryRepository) RecordReplayEvent(_ context.Context, result EventResult) error {
	repo.results = append(repo.results, result)
	return nil
}
func (repo *memoryRepository) CompleteReplay(_ context.Context, run Run) error {
	repo.run = run
	return nil
}

func TestReplayAnalyzesFrozenIncidentWithoutProductionPersistence(t *testing.T) {
	now := time.Now().UTC()
	events := make([]domain.Event, 0, 4)
	for index := 0; index < 4; index++ {
		events = append(events, domain.Event{
			ID: string(rune('a' + index)), Source: "checkout", Type: "request.duration",
			Timestamp: now.Add(time.Duration(index) * time.Second), SchemaVersion: "1.0",
			Tags: map[string]string{"request_id": string(rune('A' + index)), "region": "us-west"},
		})
	}
	repo := &memoryRepository{events: events}
	active := policy.Policy{
		Name: "active", Version: "1", DefaultUniqueThreshold: 1000, DefaultAction: policy.ActionAllow,
		DangerousKeys: []string{"request_id"},
		Rules:         []policy.Rule{{Source: "*", Type: "*", Dimension: "request_id", UniqueThreshold: 3, Action: policy.ActionDropTag}},
	}
	run, err := NewRunner(repo).Run(context.Background(), Options{IncidentID: "INC-1", ActivePolicy: active, MaxEvents: 10, MaxDimensions: 100})
	if err != nil {
		t.Fatal(err)
	}
	if run.EventCount != 4 {
		t.Fatalf("events=%d, want 4", run.EventCount)
	}
	if run.DroppedTagCount == 0 {
		t.Fatal("expected replay to detect dropped request_id tags")
	}
	if len(repo.results) != 4 {
		t.Fatalf("results=%d, want 4", len(repo.results))
	}
	if repo.run.Status != "completed" {
		t.Fatalf("status=%q, want completed", repo.run.Status)
	}
}

type memoryPublisher struct {
	topics []string
}

func (publisher *memoryPublisher) Publish(_ context.Context, topic string, _ domain.Event) error {
	publisher.topics = append(publisher.topics, topic)
	return nil
}

func TestReplayRejectsProductionTopicAtLibraryBoundary(t *testing.T) {
	repo := &memoryRepository{events: []domain.Event{{
		ID: "evt", Source: "checkout", Type: "request.duration",
		Timestamp: time.Now().UTC(), SchemaVersion: "1.0",
	}}}
	active := policy.Policy{
		Name: "active", Version: "1",
		DefaultUniqueThreshold: 1000,
		DefaultAction:          policy.ActionAllow,
	}
	publisher := &memoryPublisher{}

	_, err := NewRunner(repo).Run(context.Background(), Options{
		IncidentID:   "INC-1",
		ActivePolicy: active,
		PublishTopic: "telemetry.raw",
		Publisher:    publisher,
	})
	if err == nil {
		t.Fatal("expected production topic to be rejected")
	}
	if len(publisher.topics) != 0 {
		t.Fatal("unsafe replay topic must never receive a publish")
	}
}

func TestReplayCanPublishToDedicatedReplayNamespace(t *testing.T) {
	repo := &memoryRepository{events: []domain.Event{{
		ID: "evt", Source: "checkout", Type: "request.duration",
		Timestamp: time.Now().UTC(), SchemaVersion: "1.0",
		Tags: map[string]string{"region": "us"},
	}}}
	active := policy.Policy{
		Name: "active", Version: "1",
		DefaultUniqueThreshold: 1000,
		DefaultAction:          policy.ActionAllow,
	}
	publisher := &memoryPublisher{}

	run, err := NewRunner(repo).Run(context.Background(), Options{
		IncidentID:   "INC-1",
		ActivePolicy: active,
		PublishTopic: "telemetry.replay.policy-test",
		Publisher:    publisher,
	})
	if err != nil {
		t.Fatal(err)
	}
	if run.PublishedCount != 1 || len(publisher.topics) != 1 {
		t.Fatalf("published=%d topics=%d, want 1/1", run.PublishedCount, len(publisher.topics))
	}
}
