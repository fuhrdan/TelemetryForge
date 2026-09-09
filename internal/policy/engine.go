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
	RecordCardinalityBudgetStatus(ctx context.Context, status BudgetStatus) error
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
	activeTracker CardinalityTracker
	shadow        *Policy
	shadowTracker CardinalityTracker
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

// NewEngine creates a local cardinality-policy engine.
//
// Replay, cost simulation, and unit tests use this constructor deliberately so
// historical analysis cannot mutate production distributed cardinality state.
func NewEngine(active Policy, shadow *Policy, recorder Recorder, maxDimensions int) (*Engine, error) {
	activeTracker := CardinalityTracker(NewTracker(maxDimensions))
	var shadowTracker CardinalityTracker
	if shadow != nil {
		shadowTracker = NewTracker(maxDimensions)
	}
	return NewEngineWithTrackers(active, shadow, recorder, activeTracker, shadowTracker, maxDimensions)
}

// NewEngineWithTrackers creates a policy engine with explicit cardinality
// state providers. Production v1.2 workers inject shared PostgreSQL-backed
// trackers for active/shadow policy.
func NewEngineWithTrackers(
	active Policy,
	shadow *Policy,
	recorder Recorder,
	activeTracker CardinalityTracker,
	shadowTracker CardinalityTracker,
	maxDimensions int,
) (*Engine, error) {
	if err := active.Validate(); err != nil {
		return nil, err
	}
	if activeTracker == nil {
		return nil, fmt.Errorf("active cardinality tracker is required")
	}
	if shadow != nil {
		if err := shadow.Validate(); err != nil {
			return nil, fmt.Errorf("shadow policy: %w", err)
		}
		if shadowTracker == nil {
			return nil, fmt.Errorf("shadow cardinality tracker is required when shadow policy is configured")
		}
	}

	engine := &Engine{
		active:           active,
		activeTracker:    activeTracker,
		shadow:           shadow,
		shadowTracker:    shadowTracker,
		recorder:         recorder,
		lastReport:       make(map[reportKey]time.Time),
		maxReportEntries: maxDimensions * 4,
	}
	if engine.maxReportEntries < 100 {
		engine.maxReportEntries = 100
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
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}

	if err := engine.evaluateBudgets(ctx, engine.active, engine.activeTracker, "active", event, now); err != nil {
		return domain.Event{}, err
	}
	if engine.shadow != nil {
		if err := engine.evaluateBudgets(ctx, *engine.shadow, engine.shadowTracker, "shadow", event, now); err != nil {
			return domain.Event{}, err
		}
	}
	if len(event.Tags) == 0 {
		return event, nil
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
		activeFinding, err := engine.evaluate(
			ctx,
			engine.active,
			engine.activeTracker,
			"active",
			event,
			dimension,
			value,
			now,
		)
		if err != nil {
			return domain.Event{}, err
		}

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
			shadowFinding, err := engine.evaluate(
				ctx,
				*engine.shadow,
				engine.shadowTracker,
				"shadow",
				event,
				dimension,
				value,
				now,
			)
			if err != nil {
				return domain.Event{}, err
			}

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
	ctx context.Context,
	policy Policy,
	tracker CardinalityTracker,
	mode string,
	event domain.Event,
	dimension, value string,
	now time.Time,
) (*Finding, error) {
	threshold, action, dangerous := policy.Decision(event.Source, event.Type, dimension)
	observation, err := tracker.ObserveCardinality(
		ctx, event.TenantID, event.Source, event.Type, dimension, value, now,
	)
	if err != nil {
		return nil, fmt.Errorf("observe cardinality: %w", err)
	}
	observed := observation.ObservedUnique
	projected := observation.ProjectedUnique

	thresholdCrossed := observed >= threshold || projected >= threshold
	earlyDangerousWarning := dangerous &&
		observed >= maxUint64(3, threshold/10) &&
		!thresholdCrossed

	if !thresholdCrossed && !earlyDangerousWarning {
		return nil, nil
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
		ValueFingerprint: observation.Fingerprint,
		FirstSeen:        observation.FirstSeen,
		LastSeen:         observation.LastSeen,
	}, nil
}

const budgetSeriesDimension = "__series__"

func (engine *Engine) evaluateBudgets(
	ctx context.Context,
	policy Policy,
	tracker CardinalityTracker,
	mode string,
	event domain.Event,
	now time.Time,
) error {
	budgets := policy.MatchingBudgets(event.Source, event.Type)
	if len(budgets) == 0 {
		return nil
	}

	for _, budget := range budgets {
		// Budget state is keyed by budget name rather than one concrete source.
		// That lets a tenant-wide `* / *` budget aggregate series across every
		// matching source/type while a narrower budget aggregates only its scope.
		observation, err := tracker.ObserveCardinality(
			ctx, event.TenantID, "__budget__", budget.Name,
			budgetSeriesDimension, seriesIdentity(event), now,
		)
		if err != nil {
			return fmt.Errorf("observe series budget %q: %w", budget.Name, err)
		}

		risk := observation.ProjectedUnique
		if observation.ObservedUnique > risk {
			risk = observation.ObservedUnique
		}
		percent := 100 * float64(risk) / float64(budget.SeriesLimit)
		status := "healthy"
		switch {
		case percent >= 100:
			status = "exceeded"
		case percent >= budget.CriticalPercent:
			status = "critical"
		case percent >= budget.WarningPercent:
			status = "warning"
		}

		budgetStatus := BudgetStatus{
			TenantID: event.TenantID, PolicyName: policy.Name,
			PolicyVersion: policy.Version, Mode: mode, BudgetName: budget.Name,
			Source: budget.Source, EventType: budget.Type,
			WindowStart: observation.WindowStart, SeriesLimit: budget.SeriesLimit,
			ObservedUnique:     observation.ObservedUnique,
			ProjectedUnique:    observation.ProjectedUnique,
			ConsumptionPercent: percent, Status: status, ObservedAt: now,
		}
		if engine.recorder != nil && engine.shouldReportBudget(budgetStatus, now) {
			if err := engine.recorder.RecordCardinalityBudgetStatus(ctx, budgetStatus); err != nil {
				return fmt.Errorf("record cardinality budget status: %w", err)
			}
		}
	}
	return nil
}

func seriesIdentity(event domain.Event) string {
	keys := make([]string, 0, len(event.Tags))
	for key := range event.Tags {
		if strings.HasPrefix(strings.ToLower(key), "telemetryforge.") {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var builder strings.Builder
	builder.WriteString(event.Source)
	builder.WriteByte('|')
	builder.WriteString(event.Type)
	for _, key := range keys {
		builder.WriteByte('|')
		builder.WriteString(key)
		builder.WriteByte('=')
		builder.WriteString(event.Tags[key])
	}
	return builder.String()
}

func (engine *Engine) shouldReportBudget(status BudgetStatus, now time.Time) bool {
	return engine.reserveReportInterval(reportKey{
		tenant: status.TenantID, mode: "budget:" + status.Mode,
		source: status.Source, eventType: status.EventType,
		dimension: status.BudgetName, action: Action(status.Status),
	}, now, 10*time.Second)
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
	return engine.reserveReportInterval(key, now, time.Minute)
}

func (engine *Engine) reserveReportInterval(key reportKey, now time.Time, interval time.Duration) bool {
	engine.reportMu.Lock()
	defer engine.reportMu.Unlock()

	if interval <= 0 {
		interval = time.Minute
	}
	if last := engine.lastReport[key]; !last.IsZero() && now.Sub(last) < interval {
		return false
	}

	// Keep reporting metadata bounded by the same order of magnitude as tracked
	// dimensions. Old entries can be dropped because they only suppress duplicate
	// findings/status snapshots; dropping one may emit an extra record but cannot
	// alter policy behavior.
	if len(engine.lastReport) > engine.maxReportEntries {
		cutoff := now.Add(-time.Minute)
		for existing, timestamp := range engine.lastReport {
			if timestamp.Before(cutoff) {
				delete(engine.lastReport, existing)
			}
		}
	}
	engine.lastReport[key] = now
	return true
}
