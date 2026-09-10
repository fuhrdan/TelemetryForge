// Package changeintel models deployment/change evidence and before/after
// analysis without converting temporal correlation into causal certainty.
package changeintel

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
)

const (
	KindDeployment     = "deployment"
	KindRollback       = "rollback"
	KindRelease        = "release"
	KindFeatureFlag    = "feature_flag"
	KindConfiguration  = "configuration"
	KindInfrastructure = "infrastructure"

	AssessmentRegressed    = "regressed"
	AssessmentImproved     = "improved"
	AssessmentUnchanged    = "unchanged"
	AssessmentInsufficient = "insufficient"
)

// Marker is one normalized operational change captured from telemetry.
type Marker struct {
	TenantID        string    `json:"tenant_id"`
	ChangeID        string    `json:"change_id"`
	EventID         string    `json:"event_id"`
	Source          string    `json:"source"`
	Kind            string    `json:"kind"`
	Status          string    `json:"status"`
	Environment     string    `json:"environment,omitempty"`
	Version         string    `json:"version,omitempty"`
	PreviousVersion string    `json:"previous_version,omitempty"`
	GitSHA          string    `json:"git_sha,omitempty"`
	BuildID         string    `json:"build_id,omitempty"`
	Actor           string    `json:"actor,omitempty"`
	RollbackOf      string    `json:"rollback_of,omitempty"`
	Summary         string    `json:"summary,omitempty"`
	ChangedAt       time.Time `json:"changed_at"`
	RecordedAt      time.Time `json:"recorded_at,omitempty"`
}

// WindowStats is one source's observed telemetry shape in a bounded window.
type WindowStats struct {
	Events       int64   `json:"events"`
	Errors       int64   `json:"errors"`
	ErrorRate    float64 `json:"error_rate"`
	P95LatencyMS float64 `json:"p95_latency_ms"`
}

// SourceImpact compares one observed source before and after a change.
type SourceImpact struct {
	Source         string      `json:"source"`
	Before         WindowStats `json:"before"`
	After          WindowStats `json:"after"`
	ErrorRateDelta float64     `json:"error_rate_delta"`
	P95DeltaMS     float64     `json:"p95_delta_ms"`
	Assessment     string      `json:"assessment"`
	Reason         string      `json:"reason"`
}

// RecoveryEvidence records a later rollback/change and the resulting source
// signal. It is evidence for recovery timing, not proof that rollback caused it.
type RecoveryEvidence struct {
	Rollback      *Marker     `json:"rollback,omitempty"`
	AfterRollback WindowStats `json:"after_rollback"`
	Recovered     bool        `json:"recovered"`
	Reason        string      `json:"reason,omitempty"`
}

// Analysis is a bounded before/after comparison for one change marker.
type Analysis struct {
	TenantID            string           `json:"tenant_id"`
	Change              Marker           `json:"change"`
	GeneratedAt         time.Time        `json:"generated_at"`
	BeforeWindowSeconds int64            `json:"before_window_seconds"`
	AfterWindowSeconds  int64            `json:"after_window_seconds"`
	Assessment          string           `json:"assessment"`
	Disclaimer          string           `json:"disclaimer"`
	PrimarySource       SourceImpact     `json:"primary_source"`
	SourceImpacts       []SourceImpact   `json:"source_impacts"`
	ObservedSources     int              `json:"observed_sources"`
	AssessableSources   int              `json:"assessable_sources"`
	RegressedSources    int              `json:"regressed_sources"`
	ImprovedSources     int              `json:"improved_sources"`
	BlastRadiusPercent  float64          `json:"blast_radius_percent"`
	Recovery            RecoveryEvidence `json:"recovery"`
	Notes               []string         `json:"notes"`
}

// IsChangeEvent recognizes the focused change vocabulary used by v1.7 while
// retaining compatibility with earlier deployment/release marker conventions.
func IsChangeEvent(event domain.Event) bool {
	kind := strings.ToLower(strings.TrimSpace(event.Type))
	if strings.Contains(kind, "deploy") || strings.Contains(kind, "rollback") ||
		strings.Contains(kind, "release") || strings.Contains(kind, "change") {
		return true
	}
	for _, key := range []string{
		"telemetryforge.change_id", "change_id", "deployment_id", "rollback_of",
		"git_sha", "build_id", "release", "version",
	} {
		if strings.TrimSpace(event.Tags[key]) != "" {
			return true
		}
	}
	return false
}

// MarkerFromEvent converts one canonical change event into structured evidence.
func MarkerFromEvent(event domain.Event) (Marker, bool) {
	if !IsChangeEvent(event) {
		return Marker{}, false
	}
	marker := Marker{
		TenantID:        event.TenantID,
		EventID:         event.ID,
		Source:          event.Source,
		Kind:            inferKind(event),
		Status:          first(event.Tags, "telemetryforge.change_status", "change_status", "status"),
		Environment:     first(event.Tags, "telemetryforge.environment", "environment", "deployment.environment"),
		Version:         first(event.Tags, "telemetryforge.version", "version", "release"),
		PreviousVersion: first(event.Tags, "telemetryforge.previous_version", "previous_version"),
		GitSHA:          first(event.Tags, "telemetryforge.git_sha", "git_sha", "git.commit.sha", "vcs.revision"),
		BuildID:         first(event.Tags, "telemetryforge.build_id", "build_id"),
		Actor:           first(event.Tags, "telemetryforge.change_actor", "actor"),
		RollbackOf:      first(event.Tags, "telemetryforge.rollback_of", "rollback_of"),
		Summary:         first(event.Tags, "telemetryforge.change_summary", "change_summary"),
		ChangedAt:       event.Timestamp.UTC(),
	}
	marker.ChangeID = first(event.Tags, "telemetryforge.change_id", "change_id", "deployment_id")
	if marker.ChangeID == "" {
		marker.ChangeID = "CHG-" + event.ID
	}
	if marker.Status == "" {
		marker.Status = "completed"
	}
	if marker.Summary == "" && len(event.Payload) > 0 {
		var payload struct {
			Summary string `json:"summary"`
		}
		if json.Unmarshal(event.Payload, &payload) == nil {
			marker.Summary = strings.TrimSpace(payload.Summary)
		}
	}
	return marker, true
}

// Validate enforces the change identity used by persistence/API boundaries.
func (marker Marker) Validate() error {
	if strings.TrimSpace(marker.ChangeID) == "" {
		return errors.New("change_id is required")
	}
	if strings.TrimSpace(marker.EventID) == "" {
		return errors.New("event_id is required")
	}
	if strings.TrimSpace(marker.Source) == "" {
		return errors.New("source is required")
	}
	if marker.ChangedAt.IsZero() {
		return errors.New("changed_at is required")
	}
	switch marker.Kind {
	case KindDeployment, KindRollback, KindRelease, KindFeatureFlag, KindConfiguration, KindInfrastructure:
	default:
		return fmt.Errorf("unsupported change kind %q", marker.Kind)
	}
	if strings.TrimSpace(marker.Status) == "" {
		return errors.New("status is required")
	}
	switch marker.Status {
	case "started", "completed", "failed", "rolled_back", "changed":
	default:
		return fmt.Errorf("unsupported change status %q", marker.Status)
	}
	return nil
}

// AssessImpact applies intentionally conservative materiality thresholds.
// It requires enough events on both sides before making a regression/improvement
// classification, and never returns a causal statement.
func AssessImpact(source string, before, after WindowStats) SourceImpact {
	impact := SourceImpact{Source: source, Before: before, After: after}
	impact.ErrorRateDelta = after.ErrorRate - before.ErrorRate
	impact.P95DeltaMS = after.P95LatencyMS - before.P95LatencyMS
	if before.Events < 5 || after.Events < 5 {
		impact.Assessment = AssessmentInsufficient
		impact.Reason = "Fewer than five telemetry events were observed on one side of the comparison window."
		return impact
	}

	latencyRegression := before.P95LatencyMS > 0 && impact.P95DeltaMS >= 100 && after.P95LatencyMS >= before.P95LatencyMS*1.25
	errorRegression := impact.ErrorRateDelta >= 0.05 && after.Errors >= before.Errors+2
	latencyImprovement := before.P95LatencyMS > 0 && impact.P95DeltaMS <= -100 && after.P95LatencyMS <= before.P95LatencyMS*0.75
	errorImprovement := impact.ErrorRateDelta <= -0.05 && before.Errors >= after.Errors+2

	switch {
	case latencyRegression || errorRegression:
		impact.Assessment = AssessmentRegressed
		impact.Reason = "The post-change window shows a material error-rate or p95-latency regression. This is temporal evidence, not proof of causation."
	case latencyImprovement || errorImprovement:
		impact.Assessment = AssessmentImproved
		impact.Reason = "The post-change window shows a material error-rate or p95-latency improvement."
	default:
		impact.Assessment = AssessmentUnchanged
		impact.Reason = "No material error-rate or p95-latency change crossed the conservative v1.7 thresholds."
	}
	return impact
}

// Finalize calculates bounded blast-radius and overall assessment from source impacts.
func (analysis *Analysis) Finalize() {
	analysis.Disclaimer = "Before/after relationships are observational evidence. TelemetryForge does not claim that a change caused an incident solely because it preceded degradation."
	sort.Slice(analysis.SourceImpacts, func(i, j int) bool {
		rank := map[string]int{AssessmentRegressed: 0, AssessmentImproved: 1, AssessmentUnchanged: 2, AssessmentInsufficient: 3}
		if rank[analysis.SourceImpacts[i].Assessment] == rank[analysis.SourceImpacts[j].Assessment] {
			return analysis.SourceImpacts[i].Source < analysis.SourceImpacts[j].Source
		}
		return rank[analysis.SourceImpacts[i].Assessment] < rank[analysis.SourceImpacts[j].Assessment]
	})
	analysis.ObservedSources = len(analysis.SourceImpacts)
	for _, impact := range analysis.SourceImpacts {
		switch impact.Assessment {
		case AssessmentRegressed:
			analysis.RegressedSources++
			analysis.AssessableSources++
		case AssessmentImproved:
			analysis.ImprovedSources++
			analysis.AssessableSources++
		case AssessmentUnchanged:
			analysis.AssessableSources++
		}
		if impact.Source == analysis.Change.Source {
			analysis.PrimarySource = impact
		}
	}
	if analysis.AssessableSources > 0 {
		analysis.BlastRadiusPercent = float64(analysis.RegressedSources) / float64(analysis.AssessableSources) * 100
	}
	switch {
	case analysis.PrimarySource.Assessment == AssessmentRegressed && analysis.RegressedSources > 0:
		analysis.Assessment = "regression-associated"
	case analysis.RegressedSources > 0 && analysis.ImprovedSources > 0:
		analysis.Assessment = "mixed"
	case analysis.RegressedSources > 0:
		analysis.Assessment = "regression-observed"
	case analysis.PrimarySource.Assessment == AssessmentImproved:
		analysis.Assessment = "improvement-associated"
	case analysis.AssessableSources == 0:
		analysis.Assessment = "insufficient-evidence"
	default:
		analysis.Assessment = "no-material-regression"
	}
}

func inferKind(event domain.Event) string {
	value := strings.ToLower(first(event.Tags, "telemetryforge.change_kind", "change_kind"))
	switch value {
	case KindDeployment, KindRollback, KindRelease, KindFeatureFlag, KindConfiguration, KindInfrastructure:
		return value
	}
	eventType := strings.ToLower(event.Type)
	switch {
	case strings.Contains(eventType, "rollback"):
		return KindRollback
	case strings.Contains(eventType, "deploy"):
		return KindDeployment
	case strings.Contains(eventType, "release"):
		return KindRelease
	case strings.Contains(eventType, "feature") && strings.Contains(eventType, "flag"):
		return KindFeatureFlag
	case strings.Contains(eventType, "config"):
		return KindConfiguration
	default:
		return KindInfrastructure
	}
}

func first(tags map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(tags[key]); value != "" {
			return value
		}
	}
	return ""
}
