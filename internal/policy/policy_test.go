package policy

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
)

type memoryRecorder struct {
	findings   []Finding
	diffs      []Diff
	quarantine int
}

func (recorder *memoryRecorder) RecordCardinalityFinding(_ context.Context, finding Finding) error {
	recorder.findings = append(recorder.findings, finding)
	return nil
}

func (recorder *memoryRecorder) RecordPolicyDiff(_ context.Context, diff Diff) error {
	recorder.diffs = append(recorder.diffs, diff)
	return nil
}

func (recorder *memoryRecorder) RecordQuarantine(_ context.Context, _ domain.Event, _ string) error {
	recorder.quarantine++
	return nil
}

func testPolicy(action Action, threshold uint64) Policy {
	return Policy{
		Name:                   "test",
		Version:                "1",
		DefaultUniqueThreshold: threshold,
		DefaultAction:          action,
		DangerousKeys:          []string{"request_id", "session_id"},
	}
}

func TestDropTagPolicyRemovesDimension(t *testing.T) {
	recorder := &memoryRecorder{}
	engine, err := NewEngine(testPolicy(ActionDropTag, 2), nil, recorder, 100)
	if err != nil {
		t.Fatal(err)
	}

	event := domain.Event{
		Source: "checkout",
		Type:   "request.duration",
		Tags:   map[string]string{"request_id": "A"},
	}
	if _, err := engine.Evaluate(context.Background(), event); err != nil {
		t.Fatal(err)
	}

	event.Tags["request_id"] = "B"
	processed, err := engine.Evaluate(context.Background(), event)
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := processed.Tags["request_id"]; exists {
		t.Fatal("request_id should be removed by active drop_tag policy")
	}
	if len(recorder.findings) == 0 {
		t.Fatal("expected cardinality finding")
	}
}

func TestQuarantineMarksEventAndPreservesEvidence(t *testing.T) {
	recorder := &memoryRecorder{}
	engine, err := NewEngine(testPolicy(ActionQuarantine, 2), nil, recorder, 100)
	if err != nil {
		t.Fatal(err)
	}

	event := domain.Event{
		Source: "checkout",
		Type:   "request.duration",
		Tags:   map[string]string{"session_id": "one"},
	}
	_, _ = engine.Evaluate(context.Background(), event)
	event.Tags["session_id"] = "two"
	processed, err := engine.Evaluate(context.Background(), event)
	if err != nil {
		t.Fatal(err)
	}
	if !IsQuarantined(processed) {
		t.Fatal("expected event to be marked quarantined")
	}
	if recorder.quarantine != 1 {
		t.Fatalf("quarantine records=%d, want 1", recorder.quarantine)
	}
}

func TestShadowPolicyDoesNotMutateActiveEvent(t *testing.T) {
	active := testPolicy(ActionAllow, 10000)
	shadow := testPolicy(ActionDropTag, 2)
	shadow.Name = "candidate"
	recorder := &memoryRecorder{}

	engine, err := NewEngine(active, &shadow, recorder, 100)
	if err != nil {
		t.Fatal(err)
	}

	event := domain.Event{
		Source: "checkout",
		Type:   "request.duration",
		Tags:   map[string]string{"request_id": "one"},
	}
	_, _ = engine.Evaluate(context.Background(), event)
	event.Tags["request_id"] = "two"

	processed, err := engine.Evaluate(context.Background(), event)
	if err != nil {
		t.Fatal(err)
	}
	if processed.Tags["request_id"] != "two" {
		t.Fatal("shadow policy must not mutate active event")
	}
	if len(recorder.diffs) == 0 {
		t.Fatal("expected active/shadow policy difference")
	}
}

func TestTrackerStateIsBounded(t *testing.T) {
	tracker := NewTracker(3)
	for index := 0; index < 20; index++ {
		tracker.Observe(
			"tenant-a",
			"source",
			"metric",
			string(rune('a'+index)),
			"value",
			time.Now().UTC(),
		)
	}
	if len(tracker.states) > 3 {
		t.Fatalf("tracked dimensions=%d, want <=3", len(tracker.states))
	}
}

func TestLoadRejectsUnknownField(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "policy.json")
	payload := `{
	  "name":"bad",
	  "version":"1",
	  "default_unique_threshold":100,
	  "default_action":"allow",
	  "dangerous_keys":[],
	  "unexpected":true
	}`
	if err := os.WriteFile(filename, []byte(payload), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(filename); err == nil {
		t.Fatal("expected unknown policy field to fail validation")
	}
}

func TestRepeatedFindingIsRateLimitedButActionStillApplies(t *testing.T) {
	recorder := &memoryRecorder{}
	engine, err := NewEngine(testPolicy(ActionDropTag, 2), nil, recorder, 100)
	if err != nil {
		t.Fatal(err)
	}

	for index := 0; index < 20; index++ {
		event := domain.Event{
			Source: "checkout",
			Type:   "request.duration",
			Tags:   map[string]string{"request_id": string(rune('a' + index))},
		}
		processed, err := engine.Evaluate(context.Background(), event)
		if err != nil {
			t.Fatal(err)
		}
		if index > 1 {
			if _, exists := processed.Tags["request_id"]; exists {
				t.Fatal("rate-limited finding must not disable active drop_tag enforcement")
			}
		}
	}

	if len(recorder.findings) > 2 {
		t.Fatalf("finding records=%d, expected duplicate suppression", len(recorder.findings))
	}
}

func TestEvaluateAtUsesExplicitObservationTimeline(t *testing.T) {
	recorder := &memoryRecorder{}
	active := testPolicy(ActionAllow, 20)
	engine, err := NewEngine(active, nil, recorder, 100)
	if err != nil {
		t.Fatal(err)
	}

	start := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	for index := 0; index < 10; index++ {
		event := domain.Event{
			Source: "checkout",
			Type:   "request.duration",
			Tags:   map[string]string{"region": string(rune('a' + index))},
		}
		if _, err := engine.EvaluateAt(
			context.Background(),
			event,
			start.Add(time.Duration(index)*10*time.Second),
		); err != nil {
			t.Fatal(err)
		}
	}

	if len(recorder.findings) == 0 {
		t.Fatal("expected explicit replay timeline to permit projected-cardinality finding")
	}
}

func TestCardinalityStateIsTenantIsolated(t *testing.T) {
	tracker := NewTracker(100)
	now := time.Now().UTC()

	alpha, _, _, _, _ := tracker.Observe(
		"alpha", "checkout", "request.duration", "request_id", "same-id", now,
	)
	beta, _, _, _, _ := tracker.Observe(
		"beta", "checkout", "request.duration", "request_id", "different-id", now,
	)

	if alpha != 1 || beta != 1 {
		t.Fatalf("tenant counts alpha=%d beta=%d, want 1/1", alpha, beta)
	}
	if len(tracker.states) != 2 {
		t.Fatalf("tracked states=%d, want separate state per tenant", len(tracker.states))
	}
}
