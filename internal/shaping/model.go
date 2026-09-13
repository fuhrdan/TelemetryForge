// Package shaping implements deterministic adaptive sampling and telemetry
// shaping without mutating the Incident Flight Recorder source of truth.
package shaping

import (
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
)

const (
	PayloadKeep = "keep"
	PayloadDrop = "drop"

	maxConfigIdentityLength = 128
	maxRules                = 256
	maxRuleNameLength       = 128
	maxRuleTagMatchers      = 128
	maxRuleTransforms       = 128
)

// Config is one versioned adaptive-sampling and shaping policy.
type Config struct {
	Name              string         `json:"name"`
	Version           string         `json:"version"`
	DefaultSampleRate float64        `json:"default_sample_rate"`
	Protection        Protection     `json:"protection"`
	Pressure          PressureConfig `json:"pressure"`
	Rules             []Rule         `json:"rules,omitempty"`
}

// Protection defines events that must never be sampled out by the active
// policy. Protected events can still receive explicitly configured safe tag
// transformations only when a matching rule requests them.
type Protection struct {
	Errors             bool              `json:"errors"`
	Severities         []string          `json:"severities,omitempty"`
	Types              []string          `json:"types,omitempty"`
	Tags               map[string]string `json:"tags,omitempty"`
	LatencyThresholdMS float64           `json:"latency_threshold_ms,omitempty"`
}

// PressureConfig scales sampling rates from bounded worker queue utilization.
type PressureConfig struct {
	Enabled           bool    `json:"enabled"`
	HighWatermark     float64 `json:"high_watermark"`
	CriticalWatermark float64 `json:"critical_watermark"`
	HighFactor        float64 `json:"high_factor"`
	CriticalFactor    float64 `json:"critical_factor"`
}

// Rule composes transformations and optionally overrides the sample rate for a
// matching event. Rules are evaluated in order until Stop is true.
type Rule struct {
	Name            string            `json:"name"`
	Tenant          string            `json:"tenant,omitempty"`
	Source          string            `json:"source,omitempty"`
	Type            string            `json:"type,omitempty"`
	Severity        []string          `json:"severity,omitempty"`
	Tags            map[string]string `json:"tags,omitempty"`
	SampleRate      *float64          `json:"sample_rate,omitempty"`
	MinSampleRate   float64           `json:"min_sample_rate,omitempty"`
	DropTags        []string          `json:"drop_tags,omitempty"`
	RenameTags      map[string]string `json:"rename_tags,omitempty"`
	PayloadMaxBytes int               `json:"payload_max_bytes,omitempty"`
	PayloadAction   string            `json:"payload_action,omitempty"`
	ShapeProtected  bool              `json:"shape_protected,omitempty"`
	Stop            bool              `json:"stop,omitempty"`
}

// Decision is the explainable result for one event.
type Decision struct {
	TenantID           string            `json:"tenant_id,omitempty"`
	EventID            string            `json:"event_id"`
	Source             string            `json:"source"`
	EventType          string            `json:"event_type"`
	ObservedAt         time.Time         `json:"observed_at"`
	ConfigName         string            `json:"config_name"`
	ConfigVersion      string            `json:"config_version"`
	Rules              []string          `json:"rules,omitempty"`
	Keep               bool              `json:"keep"`
	Protected          bool              `json:"protected"`
	ProtectionReason   string            `json:"protection_reason,omitempty"`
	BaseRate           float64           `json:"base_rate"`
	EffectiveRate      float64           `json:"effective_rate"`
	QueuePressure      float64           `json:"queue_pressure"`
	AutonomyMultiplier float64           `json:"autonomy_multiplier,omitempty"`
	DroppedTags        []string          `json:"dropped_tags,omitempty"`
	RenamedTags        map[string]string `json:"renamed_tags,omitempty"`
	PayloadDropped     bool              `json:"payload_dropped"`
	OriginalBytes      int               `json:"original_bytes"`
	ShapedBytes        int               `json:"shaped_bytes"`
	Reason             string            `json:"reason"`
}

// ShadowDiff records a candidate shaping disagreement without mutating the
// active event path.
type ShadowDiff struct {
	TenantID      string    `json:"tenant_id,omitempty"`
	EventID       string    `json:"event_id"`
	ObservedAt    time.Time `json:"observed_at"`
	ActiveConfig  string    `json:"active_config"`
	ActiveVersion string    `json:"active_version"`
	ShadowConfig  string    `json:"shadow_config"`
	ShadowVersion string    `json:"shadow_version"`
	ActiveKeep    bool      `json:"active_keep"`
	ShadowKeep    bool      `json:"shadow_keep"`
	ActiveRate    float64   `json:"active_rate"`
	ShadowRate    float64   `json:"shadow_rate"`
	ActiveEffects []string  `json:"active_effects,omitempty"`
	ShadowEffects []string  `json:"shadow_effects,omitempty"`
}

// Summary is an exact SQL aggregate over the requested shaping-stat window.
// It is separate from paginated detail rows so dashboard retention KPIs do not
// depend on the detail-page limit.
type Summary struct {
	Observed       int64 `json:"observed"`
	Kept           int64 `json:"kept"`
	SampledOut     int64 `json:"sampled_out"`
	Protected      int64 `json:"protected"`
	Transformed    int64 `json:"transformed"`
	PayloadDropped int64 `json:"payload_dropped"`
	OriginalBytes  int64 `json:"original_bytes"`
	ShapedBytes    int64 `json:"shaped_bytes"`
}

// Stat is one tenant-visible minute aggregate used by the dashboard.
type Stat struct {
	Bucket         time.Time `json:"bucket"`
	TenantID       string    `json:"tenant_id,omitempty"`
	ConfigName     string    `json:"config_name"`
	ConfigVersion  string    `json:"config_version"`
	RuleName       string    `json:"rule_name"`
	Source         string    `json:"source"`
	EventType      string    `json:"event_type"`
	Observed       int64     `json:"observed"`
	Kept           int64     `json:"kept"`
	SampledOut     int64     `json:"sampled_out"`
	Protected      int64     `json:"protected"`
	Transformed    int64     `json:"transformed"`
	PayloadDropped int64     `json:"payload_dropped"`
	OriginalBytes  int64     `json:"original_bytes"`
	ShapedBytes    int64     `json:"shaped_bytes"`
}

// Preview summarizes a deterministic historical what-if run. Percentages are
// explicit retention measurements, not an opaque "AI visibility score".
type Preview struct {
	ConfigName                string  `json:"config_name"`
	ConfigVersion             string  `json:"config_version"`
	QueuePressure             float64 `json:"queue_pressure"`
	Observed                  int     `json:"observed"`
	Kept                      int     `json:"kept"`
	SampledOut                int     `json:"sampled_out"`
	ProtectedObserved         int     `json:"protected_observed"`
	ProtectedKept             int     `json:"protected_kept"`
	Transformed               int     `json:"transformed"`
	PayloadDropped            int     `json:"payload_dropped"`
	OriginalBytes             int64   `json:"original_bytes"`
	ShapedBytes               int64   `json:"shaped_bytes"`
	EventRetentionPercent     float64 `json:"event_retention_percent"`
	ByteRetentionPercent      float64 `json:"byte_retention_percent"`
	ProtectedRetentionPercent float64 `json:"protected_retention_percent"`
	SourceTypeCoveragePercent float64 `json:"source_type_coverage_percent"`
}

func (config Config) Validate() error {
	if strings.TrimSpace(config.Name) == "" {
		return fmt.Errorf("shaping config name is required")
	}
	if len(config.Name) > maxConfigIdentityLength {
		return fmt.Errorf("shaping config name exceeds %d characters", maxConfigIdentityLength)
	}
	if strings.TrimSpace(config.Version) == "" {
		return fmt.Errorf("shaping config version is required")
	}
	if len(config.Version) > maxConfigIdentityLength {
		return fmt.Errorf("shaping config version exceeds %d characters", maxConfigIdentityLength)
	}
	if len(config.Rules) > maxRules {
		return fmt.Errorf("shaping config has %d rules; maximum is %d", len(config.Rules), maxRules)
	}
	if config.DefaultSampleRate < 0 || config.DefaultSampleRate > 1 {
		return fmt.Errorf("default_sample_rate must be between 0 and 1")
	}
	if err := config.Pressure.validate(); err != nil {
		return err
	}
	for _, pattern := range config.Protection.Types {
		if err := validateGlob("protection type", pattern); err != nil {
			return err
		}
	}
	for key, pattern := range config.Protection.Tags {
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("protection tag key is empty")
		}
		if err := validateGlob("protection tag "+key, pattern); err != nil {
			return err
		}
	}
	seen := map[string]struct{}{}
	for index, rule := range config.Rules {
		if strings.TrimSpace(rule.Name) == "" {
			return fmt.Errorf("shaping rule %d name is required", index)
		}
		if len(rule.Name) > maxRuleNameLength {
			return fmt.Errorf("shaping rule %q name exceeds %d characters", rule.Name, maxRuleNameLength)
		}
		if len(rule.Tags) > maxRuleTagMatchers {
			return fmt.Errorf("shaping rule %q has too many tag matchers; maximum is %d", rule.Name, maxRuleTagMatchers)
		}
		if len(rule.DropTags)+len(rule.RenameTags) > maxRuleTransforms {
			return fmt.Errorf("shaping rule %q has too many tag transforms; maximum is %d", rule.Name, maxRuleTransforms)
		}
		if _, ok := seen[rule.Name]; ok {
			return fmt.Errorf("duplicate shaping rule %q", rule.Name)
		}
		seen[rule.Name] = struct{}{}
		for label, pattern := range map[string]string{"tenant": rule.Tenant, "source": rule.Source, "type": rule.Type} {
			if err := validateGlob("shaping rule "+label, pattern); err != nil {
				return fmt.Errorf("shaping rule %q: %w", rule.Name, err)
			}
		}
		for key, pattern := range rule.Tags {
			if strings.TrimSpace(key) == "" {
				return fmt.Errorf("shaping rule %q has empty tag key", rule.Name)
			}
			if err := validateGlob("tag "+key, pattern); err != nil {
				return fmt.Errorf("shaping rule %q: %w", rule.Name, err)
			}
		}
		if rule.SampleRate != nil && (*rule.SampleRate < 0 || *rule.SampleRate > 1) {
			return fmt.Errorf("shaping rule %q sample_rate must be between 0 and 1", rule.Name)
		}
		if rule.MinSampleRate < 0 || rule.MinSampleRate > 1 {
			return fmt.Errorf("shaping rule %q min_sample_rate must be between 0 and 1", rule.Name)
		}
		if rule.SampleRate != nil && rule.MinSampleRate > *rule.SampleRate {
			return fmt.Errorf("shaping rule %q min_sample_rate cannot exceed sample_rate", rule.Name)
		}
		if rule.PayloadMaxBytes < 0 {
			return fmt.Errorf("shaping rule %q payload_max_bytes cannot be negative", rule.Name)
		}
		if rule.PayloadAction != "" && rule.PayloadAction != PayloadKeep && rule.PayloadAction != PayloadDrop {
			return fmt.Errorf("shaping rule %q payload_action must be keep or drop", rule.Name)
		}
		dropped := map[string]struct{}{}
		for _, key := range rule.DropTags {
			key = strings.TrimSpace(key)
			if key == "" {
				return fmt.Errorf("shaping rule %q contains an empty drop_tags key", rule.Name)
			}
			dropped[key] = struct{}{}
		}
		targets := map[string]struct{}{}
		for from, to := range rule.RenameTags {
			from, to = strings.TrimSpace(from), strings.TrimSpace(to)
			if from == "" || to == "" || from == to {
				return fmt.Errorf("shaping rule %q has invalid rename %q -> %q", rule.Name, from, to)
			}
			if _, ok := dropped[from]; ok {
				return fmt.Errorf("shaping rule %q cannot both drop and rename tag %q", rule.Name, from)
			}
			if _, exists := targets[to]; exists {
				return fmt.Errorf("shaping rule %q has duplicate rename target %q", rule.Name, to)
			}
			targets[to] = struct{}{}
		}
	}
	return nil
}

func (pressure PressureConfig) validate() error {
	if !pressure.Enabled {
		return nil
	}
	if pressure.HighWatermark <= 0 || pressure.HighWatermark >= 1 {
		return fmt.Errorf("pressure high_watermark must be between 0 and 1")
	}
	if pressure.CriticalWatermark <= pressure.HighWatermark || pressure.CriticalWatermark > 1 {
		return fmt.Errorf("pressure critical_watermark must be greater than high_watermark and at most 1")
	}
	if pressure.HighFactor <= 0 || pressure.HighFactor > 1 {
		return fmt.Errorf("pressure high_factor must be between 0 and 1")
	}
	if pressure.CriticalFactor <= 0 || pressure.CriticalFactor > pressure.HighFactor {
		return fmt.Errorf("pressure critical_factor must be greater than 0 and no more than high_factor")
	}
	return nil
}

func validateGlob(label, pattern string) error {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" || pattern == "*" {
		return nil
	}
	if _, err := path.Match(pattern, "validation-probe"); err != nil {
		return fmt.Errorf("%s pattern %q is invalid: %w", label, pattern, err)
	}
	return nil
}

func glob(pattern, value string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" || pattern == "*" {
		return true
	}
	matched, err := path.Match(pattern, value)
	return err == nil && matched
}

func severity(event domain.Event) string {
	for _, key := range []string{"severity", "level"} {
		if value := strings.TrimSpace(event.Tags[key]); value != "" {
			return strings.ToLower(value)
		}
	}
	if strings.Contains(strings.ToLower(event.Type), "error") {
		return "error"
	}
	return ""
}

func ruleMatches(rule Rule, event domain.Event) bool {
	if !glob(rule.Tenant, event.TenantID) || !glob(rule.Source, event.Source) || !glob(rule.Type, event.Type) {
		return false
	}
	if len(rule.Severity) > 0 {
		actual := severity(event)
		matched := false
		for _, candidate := range rule.Severity {
			if strings.EqualFold(strings.TrimSpace(candidate), actual) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	for key, pattern := range rule.Tags {
		value, ok := event.Tags[key]
		if !ok || !glob(pattern, value) {
			return false
		}
	}
	return true
}

func effects(decision Decision) []string {
	result := make([]string, 0, len(decision.DroppedTags)+len(decision.RenamedTags)+2)
	if !decision.Keep {
		result = append(result, "sampled_out")
	}
	for _, key := range decision.DroppedTags {
		result = append(result, "drop_tag:"+key)
	}
	keys := make([]string, 0, len(decision.RenamedTags))
	for key := range decision.RenamedTags {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		result = append(result, "rename_tag:"+key+"->"+decision.RenamedTags[key])
	}
	if decision.PayloadDropped {
		result = append(result, "drop_payload")
	}
	sort.Strings(result)
	return result
}
