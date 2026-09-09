// Package replay safely reprocesses frozen incident telemetry through current
// or candidate policy without writing it back to the production ingest path.
package replay

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/policy"
	"github.com/fuhrdan/TelemetryForge/internal/worker"
)

// Repository is the durable replay-history contract.
type Repository interface {
	IncidentEvents(context.Context, string, int) ([]domain.Event, error)
	StartReplay(context.Context, Run) error
	RecordReplayEvent(context.Context, EventResult) error
	CompleteReplay(context.Context, Run) error
}

// Publisher is an optional isolated replay destination.
type Publisher interface {
	Publish(context.Context, string, domain.Event) error
}

// Run summarizes one replay execution.
type Run struct {
	ID                string     `json:"run_id"`
	IncidentID        string     `json:"incident_id"`
	Mode              string     `json:"mode"`
	Status            string     `json:"status"`
	ActivePolicy      string     `json:"active_policy"`
	ActiveVersion     string     `json:"active_version"`
	ShadowPolicy      string     `json:"shadow_policy,omitempty"`
	ShadowVersion     string     `json:"shadow_version,omitempty"`
	OutputTopic       string     `json:"output_topic,omitempty"`
	StartedAt         time.Time  `json:"started_at"`
	CompletedAt       *time.Time `json:"completed_at,omitempty"`
	EventCount        int64      `json:"event_count"`
	ChangedEventCount int64      `json:"changed_event_count"`
	DroppedTagCount   int64      `json:"dropped_tag_count"`
	QuarantinedCount  int64      `json:"quarantined_count"`
	FindingCount      int64      `json:"finding_count"`
	ShadowDiffCount   int64      `json:"shadow_diff_count"`
	PublishedCount    int64      `json:"published_count"`
	Error             string     `json:"error,omitempty"`
}

// EventResult captures the per-event policy effect without duplicating payloads.
type EventResult struct {
	RunID           string `json:"run_id"`
	EventID         string `json:"event_id"`
	Changed         bool   `json:"changed"`
	DroppedTagCount int    `json:"dropped_tag_count"`
	Quarantined     bool   `json:"quarantined"`
	FindingCount    int    `json:"finding_count"`
	ShadowDiffCount int    `json:"shadow_diff_count"`
}

// Options controls an isolated replay.
type Options struct {
	IncidentID    string
	ActivePolicy  policy.Policy
	ShadowPolicy  *policy.Policy
	MaxEvents     int
	MaxDimensions int
	PublishTopic  string
	Publisher     Publisher
}

// Runner executes incident replays.
type Runner struct {
	repository Repository
}

// NewRunner creates a replay runner.
func NewRunner(repository Repository) *Runner { return &Runner{repository: repository} }

// Run replays a frozen incident through normalization and policy only.
//
// It never invokes the Flight Recorder, primary telemetry persistence, or
// automatic incident detection. An optional publisher is permitted only for a
// dedicated replay topic selected by the caller.
func (runner *Runner) Run(ctx context.Context, options Options) (run Run, retErr error) {
	if strings.TrimSpace(options.IncidentID) == "" {
		return Run{}, fmt.Errorf("incident id is required")
	}
	if err := options.ActivePolicy.Validate(); err != nil {
		return Run{}, fmt.Errorf("active policy: %w", err)
	}
	if options.ShadowPolicy != nil {
		if err := options.ShadowPolicy.Validate(); err != nil {
			return Run{}, fmt.Errorf("shadow policy: %w", err)
		}
	}
	if options.MaxEvents < 1 || options.MaxEvents > 100000 {
		options.MaxEvents = 10000
	}
	if options.MaxDimensions < 1 {
		options.MaxDimensions = 20000
	}
	if options.PublishTopic != "" {
		topic := strings.TrimSpace(options.PublishTopic)
		if topic != "telemetry.replay" && !strings.HasPrefix(topic, "telemetry.replay.") {
			return Run{}, fmt.Errorf("replay publish topic %q is outside the telemetry.replay namespace", topic)
		}
		options.PublishTopic = topic
		if options.Publisher == nil {
			return Run{}, fmt.Errorf("publisher is required when publish topic is set")
		}
	}

	runID, err := newRunID("RPL")
	if err != nil {
		return Run{}, err
	}
	run = Run{
		ID:            runID,
		IncidentID:    options.IncidentID,
		Mode:          "analysis",
		Status:        "running",
		ActivePolicy:  options.ActivePolicy.Name,
		ActiveVersion: options.ActivePolicy.Version,
		OutputTopic:   options.PublishTopic,
		StartedAt:     time.Now().UTC(),
	}
	if options.PublishTopic != "" {
		run.Mode = "analysis+publish"
	}
	if options.ShadowPolicy != nil {
		run.ShadowPolicy = options.ShadowPolicy.Name
		run.ShadowVersion = options.ShadowPolicy.Version
	}

	if err := runner.repository.StartReplay(ctx, run); err != nil {
		return Run{}, fmt.Errorf("start replay history: %w", err)
	}
	defer func() {
		completed := time.Now().UTC()
		run.CompletedAt = &completed
		if retErr != nil {
			run.Status = "failed"
			run.Error = retErr.Error()
		} else {
			run.Status = "completed"
		}
		if err := runner.repository.CompleteReplay(context.WithoutCancel(ctx), run); err != nil && retErr == nil {
			retErr = fmt.Errorf("complete replay history: %w", err)
		}
	}()

	events, err := runner.repository.IncidentEvents(ctx, options.IncidentID, options.MaxEvents)
	if err != nil {
		return run, fmt.Errorf("load incident events: %w", err)
	}
	if len(events) == 0 {
		return run, fmt.Errorf("incident %q contains no replayable events", options.IncidentID)
	}

	recorder := &memoryRecorder{}
	engine, err := policy.NewEngine(options.ActivePolicy, options.ShadowPolicy, recorder, options.MaxDimensions)
	if err != nil {
		return run, err
	}
	normalizer := worker.Normalizer{}

	for _, original := range events {
		normalized, err := normalizer.Process(ctx, original)
		if err != nil {
			return run, fmt.Errorf("normalize event %s: %w", original.ID, err)
		}

		beforeFindings := recorder.findings
		beforeDiffs := recorder.diffs
		processed, err := engine.EvaluateAt(ctx, normalized, normalized.Timestamp)
		if err != nil {
			return run, fmt.Errorf("evaluate event %s: %w", original.ID, err)
		}

		dropped := droppedTags(normalized.Tags, processed.Tags)
		quarantined := policy.IsQuarantined(processed)
		changed := dropped > 0 || quarantined || normalized.Source != processed.Source || normalized.Type != processed.Type

		result := EventResult{
			RunID:           run.ID,
			EventID:         original.ID,
			Changed:         changed,
			DroppedTagCount: dropped,
			Quarantined:     quarantined,
			FindingCount:    recorder.findings - beforeFindings,
			ShadowDiffCount: recorder.diffs - beforeDiffs,
		}
		if err := runner.repository.RecordReplayEvent(ctx, result); err != nil {
			return run, fmt.Errorf("record replay result for %s: %w", original.ID, err)
		}

		run.EventCount++
		run.DroppedTagCount += int64(dropped)
		run.FindingCount += int64(result.FindingCount)
		run.ShadowDiffCount += int64(result.ShadowDiffCount)
		if changed {
			run.ChangedEventCount++
		}
		if quarantined {
			run.QuarantinedCount++
		}

		if options.PublishTopic != "" {
			if processed.Tags == nil {
				processed.Tags = make(map[string]string)
			}
			processed.Tags["telemetryforge.replay_run"] = run.ID
			processed.Tags["telemetryforge.replay_incident"] = run.IncidentID
			if err := options.Publisher.Publish(ctx, options.PublishTopic, processed); err != nil {
				return run, fmt.Errorf("publish replay event %s: %w", processed.ID, err)
			}
			run.PublishedCount++
		}
	}

	return run, nil
}

func droppedTags(before, after map[string]string) int {
	count := 0
	for key := range before {
		if strings.HasPrefix(key, "telemetryforge.") {
			continue
		}
		if _, exists := after[key]; !exists {
			count++
		}
	}
	return count
}

type memoryRecorder struct {
	findings int
	diffs    int
}

func (recorder *memoryRecorder) RecordCardinalityFinding(context.Context, policy.Finding) error {
	recorder.findings++
	return nil
}
func (recorder *memoryRecorder) RecordPolicyDiff(context.Context, policy.Diff) error {
	recorder.diffs++
	return nil
}
func (recorder *memoryRecorder) RecordQuarantine(context.Context, domain.Event, string) error {
	return nil
}

func newRunID(prefix string) (string, error) {
	raw := make([]byte, 6)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate run id: %w", err)
	}
	return fmt.Sprintf("%s-%d-%s", prefix, time.Now().UTC().UnixMilli(), hex.EncodeToString(raw)), nil
}
