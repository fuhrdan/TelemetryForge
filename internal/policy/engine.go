package policy

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
)

// Recorder persists active/shadow policy evidence without exposing raw tag
// values in findings.
type Recorder interface {
	RecordCardinalityFinding(ctx context.Context, finding Finding) error
	RecordPolicyDiff(ctx context.Context, diff Diff) error
	RecordQuarantine(ctx context.Context, event domain.Event, reason string) error
}

// Diff captures an active-versus-shadow policy disagreement.
type Diff struct {
	TenantID      string    `json:"tenant_id,omitempty"`
	ObservedAt    time.Time `json:"observed_at"`
	Source        string    `json:"source"`
	EventType     string    `json:"event_type"`
	Dimension     string    `json:"dimension"`
	ActivePolicy  string    `json:"active_policy"`
	ActiveVersion string    `json:"active_version"`
	ActiveAction  Action    `json:"active_action"`
	ShadowPolicy  string    `json:"shadow_policy"`
	ShadowVersion string    `json:"shadow_version"`
	ShadowAction  Action    `json:"shadow_action"`
	Reason        string    `json:"reason"`
}

// Engine applies one active policy and optionally evaluates a shadow policy.
//
// Active and shadow trackers are intentionally separate so a candidate policy
// can change tracking/threshold behavior without mutating production decisions.
type Engine struct {
	active        Policy
	activeTracker *Tracker
	shadow        *Policy
	shadowTracker *Tracker
	recorder      Recorder

	reportMu         sync.Mutex
	lastReport       map[reportKey]time.Time
	maxReportEntries int
}

type reportKey struct {
	tenant    string
	mode      string
	source    string
	eventType string
	dimension string
	action    Action
}

// NewEngine creates a cardinality-policy engine.
func NewEngine(active Policy, shadow *Policy, recorder Recorder, maxDimensions int) (*Engine, error) {
	if err := active.Validate(); err != nil {
		return nil, err
	}
	if shadow != nil {
		if err := shadow.Validate(); err != nil {
			return nil, fmt.Errorf("shadow policy: %w", err)
		}
	}

	engine := &Engine{
		active:           active,
		activeTracker:    NewTracker(maxDimensions),
		shadow:           shadow,
		recorder:         recorder,
		lastReport:       make(map[reportKey]time.Time),
		maxReportEntries: maxDimensions * 4,
	}
	if engine.maxReportEntries < 100 {
		engine.maxReportEntries = 100
	}
	if shadow != nil {
		engine.shadowTracker = NewTracker(maxDimensions)
	}
	return engine, nil
}

// Evaluate applies policy using the worker's current wall-clock observation
// time.
func (engine *Engine) Evaluate(ctx context.Context, event domain.Event) (domain.Event, error) {
	return engine.EvaluateAt(ctx, event, time.Now().UTC())
}

// EvaluateAt applies the active policy and records shadow differences using an
// explicit observation time.
//
// Production calls Evaluate. Incident replay/cost simulation call EvaluateAt
// with the frozen event timestamp so cardinality windows represent the incident
// timeline rather than replay execution speed.
//
// The Flight Recorder has already stored the incoming full-fidelity event before
// this stage, so active `drop_tag` can safely remove a dangerous dimension from
// the normal persisted/forwarded representation without destroying incident
// evidence.
func (engine *Engine) EvaluateAt(
	ctx context.Context,
	event domain.Event,
	now time.Time,
) (domain.Event, error) {
	if len(event.Tags) == 0 {
		return event, nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	dimensions := make([]string, 0, len(event.Tags))
	for dimension := range event.Tags {
		dimensions = append(dimensions, dimension)
	}
	sort.Strings(dimensions)

	processed := event
	processed.Tags = cloneTags(event.Tags)
	quarantineDimensions := make([]string, 0, 2)
	quarantineReasons := make([]string, 0, 2)

	for _, dimension := range dimensions {
		value := event.Tags[dimension]
		activeFinding := engine.evaluate(
			engine.active,
			engine.activeTracker,
			"active",
			event,
			dimension,
			value,
			now,
		)

		if activeFinding != nil {
			if engine.recorder != nil && engine.shouldReport(*activeFinding, now) {
				if err := engine.recorder.RecordCardinalityFinding(ctx, *activeFinding); err != nil {
					return domain.Event{}, fmt.Errorf("record cardinality finding: %w", err)
				}
			}

			switch activeFinding.Action {
			case ActionDropTag:
				delete(processed.Tags, dimension)
			case ActionQuarantine:
				// Preserve the complete original envelope once after all
				// dimensions have been evaluated, but remove every dangerous
				// dimension from the normal representation immediately.
				delete(processed.Tags, dimension)
				quarantineDimensions = append(quarantineDimensions, dimension)
				quarantineReasons = append(quarantineReasons, activeFinding.Reason)
			}
		}

		if engine.shadow != nil {
			shadowFinding := engine.evaluate(
				*engine.shadow,
				engine.shadowTracker,
				"shadow",
				event,
				dimension,
				value,
				now,
			)

			activeAction := ActionAllow
			if activeFinding != nil {
				activeAction = activeFinding.Action
			}
			shadowAction := ActionAllow
			if shadowFinding != nil {
				shadowAction = shadowFinding.Action
			}

			if shadowFinding != nil && engine.recorder != nil && engine.shouldReport(*shadowFinding, now) {
				if err := engine.recorder.RecordCardinalityFinding(ctx, *shadowFinding); err != nil {
					return domain.Event{}, fmt.Errorf("record shadow finding: %w", err)
				}
			}

			if activeAction != shadowAction && engine.recorder != nil &&
				engine.shouldReportDiff(event, dimension, shadowAction, now) {
				reason := "shadow policy would change the active action"
				if err := engine.recorder.RecordPolicyDiff(ctx, Diff{
					TenantID:      event.TenantID,
					ObservedAt:    now,
					Source:        event.Source,
					EventType:     event.Type,
					Dimension:     dimension,
					ActivePolicy:  engine.active.Name,
					ActiveVersion: engine.active.Version,
					ActiveAction:  activeAction,
					ShadowPolicy:  engine.shadow.Name,
					ShadowVersion: engine.shadow.Version,
					ShadowAction:  shadowAction,
					Reason:        reason,
				}); err != nil {
					return domain.Event{}, fmt.Errorf("record policy diff: %w", err)
				}
			}
		}
	}

	if len(quarantineDimensions) > 0 {
		if processed.Tags == nil {
			processed.Tags = make(map[string]string)
		}
		processed.Tags["telemetryforge.quarantined"] = "true"
		processed.Tags["telemetryforge.quarantine_dimensions"] = strings.Join(quarantineDimensions, ",")
		if engine.recorder != nil {
			if err := engine.recorder.RecordQuarantine(
				ctx,
				event,
				strings.Join(quarantineReasons, "; "),
			); err != nil {
				return domain.Event{}, fmt.Errorf("record quarantined event: %w", err)
			}
		}
	}

	return processed, nil
}

func (engine *Engine) evaluate(
	policy Policy,
	tracker *Tracker,
	mode string,
	event domain.Event,
	dimension, value string,
	now time.Time,
) *Finding {
	threshold, action, dangerous := policy.Decision(event.Source, event.Type, dimension)
	observed, projected, firstSeen, lastSeen, fingerprint := tracker.Observe(
		event.TenantID,
		event.Source,
		event.Type,
		dimension,
		value,
		now,
	)

	thresholdCrossed := observed >= threshold || projected >= threshold
	earlyDangerousWarning := dangerous &&
		observed >= maxUint64(3, threshold/10) &&
		!thresholdCrossed

	if !thresholdCrossed && !earlyDangerousWarning {
		return nil
	}

	effectiveAction := action
	reason := ""
	if earlyDangerousWarning {
		// Identifier-shaped heuristics are evidence, not permission to destroy
		// data. The configured mutation/quarantine action becomes active only
		// when the actual cardinality threshold is crossed.
		effectiveAction = ActionAllow
		reason = fmt.Sprintf(
			"identifier-shaped dimension warning: estimated cardinality %d is approaching threshold %d",
			observed,
			threshold,
		)
	} else {
		reason = fmt.Sprintf(
			"estimated cardinality %d (projected %d) meets policy threshold %d",
			observed,
			projected,
			threshold,
		)
	}

	return &Finding{
		TenantID:         event.TenantID,
		PolicyName:       policy.Name,
		PolicyVersion:    policy.Version,
		Mode:             mode,
		Source:           event.Source,
		EventType:        event.Type,
		Dimension:        dimension,
		ObservedUnique:   observed,
		ProjectedUnique:  projected,
		Action:           effectiveAction,
		Reason:           reason,
		ValueFingerprint: fingerprint,
		FirstSeen:        firstSeen,
		LastSeen:         lastSeen,
	}
}

func cloneTags(tags map[string]string) map[string]string {
	if tags == nil {
		return nil
	}
	result := make(map[string]string, len(tags))
	for key, value := range tags {
		result[key] = value
	}
	return result
}

func maxUint64(left, right uint64) uint64 {
	if left > right {
		return left
	}
	return right
}

// IsQuarantined reports whether the active pipeline marked an event as blocked
// from future downstream routing.
func IsQuarantined(event domain.Event) bool {
	return strings.EqualFold(event.Tags["telemetryforge.quarantined"], "true")
}

func (engine *Engine) shouldReport(finding Finding, now time.Time) bool {
	key := reportKey{
		tenant:    finding.TenantID,
		mode:      finding.Mode,
		source:    finding.Source,
		eventType: finding.EventType,
		dimension: finding.Dimension,
		action:    finding.Action,
	}
	return engine.reserveReport(key, now)
}

func (engine *Engine) shouldReportDiff(
	event domain.Event,
	dimension string,
	shadowAction Action,
	now time.Time,
) bool {
	return engine.reserveReport(reportKey{
		tenant:    event.TenantID,
		mode:      "diff",
		source:    event.Source,
		eventType: event.Type,
		dimension: dimension,
		action:    shadowAction,
	}, now)
}

func (engine *Engine) reserveReport(key reportKey, now time.Time) bool {
	engine.reportMu.Lock()
	defer engine.reportMu.Unlock()

	const reportInterval = time.Minute
	if last := engine.lastReport[key]; !last.IsZero() && now.Sub(last) < reportInterval {
		return false
	}

	// Keep reporting metadata bounded by the same order of magnitude as tracked
	// dimensions. Old entries can be dropped because they only suppress duplicate
	// findings; dropping them may emit an extra finding but cannot alter policy.
	if len(engine.lastReport) > engine.maxReportEntries {
		cutoff := now.Add(-reportInterval)
		for existing, timestamp := range engine.lastReport {
			if timestamp.Before(cutoff) {
				delete(engine.lastReport, existing)
			}
		}
	}
	engine.lastReport[key] = now
	return true
}
