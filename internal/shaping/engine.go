package shaping

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
)

// Engine evaluates one active shaping policy and optional shadow candidate.
type Engine struct {
	active Config
	shadow *Config
}

func NewEngine(active Config, shadow *Config) (*Engine, error) {
	if err := active.Validate(); err != nil {
		return nil, fmt.Errorf("active shaping config: %w", err)
	}
	if shadow != nil {
		if err := shadow.Validate(); err != nil {
			return nil, fmt.Errorf("shadow shaping config: %w", err)
		}
	}
	return &Engine{active: active, shadow: shadow}, nil
}

func (engine *Engine) Active() Config  { return engine.active }
func (engine *Engine) Shadow() *Config { return engine.shadow }

// Evaluate returns the active result and an optional non-destructive shadow diff.
func (engine *Engine) Evaluate(event domain.Event, queuePressure float64, now time.Time) (domain.Event, Decision, *ShadowDiff) {
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	activeEvent, activeDecision := evaluateConfig(engine.active, event, queuePressure, now)
	if engine.shadow == nil {
		return activeEvent, activeDecision, nil
	}
	_, shadowDecision := evaluateConfig(*engine.shadow, event, queuePressure, now)
	activeEffects, shadowEffects := effects(activeDecision), effects(shadowDecision)
	if activeDecision.Keep == shadowDecision.Keep && activeDecision.EffectiveRate == shadowDecision.EffectiveRate && equalStrings(activeEffects, shadowEffects) {
		return activeEvent, activeDecision, nil
	}
	diff := &ShadowDiff{TenantID: event.TenantID, EventID: event.ID, ObservedAt: now, ActiveConfig: engine.active.Name, ActiveVersion: engine.active.Version, ShadowConfig: engine.shadow.Name, ShadowVersion: engine.shadow.Version, ActiveKeep: activeDecision.Keep, ShadowKeep: shadowDecision.Keep, ActiveRate: activeDecision.EffectiveRate, ShadowRate: shadowDecision.EffectiveRate, ActiveEffects: activeEffects, ShadowEffects: shadowEffects}
	return activeEvent, activeDecision, diff
}

func evaluateConfig(config Config, event domain.Event, queuePressure float64, now time.Time) (domain.Event, Decision) {
	originalBytes := canonicalBytes(event)
	protected, protectionReason := isProtected(config.Protection, event)
	rate := config.DefaultSampleRate
	minRate := 0.0
	shaped := cloneEvent(event)
	matched := make([]string, 0, 2)
	dropped := map[string]struct{}{}
	renamed := map[string]string{}
	payloadDropped := false

	for _, rule := range config.Rules {
		if !ruleMatches(rule, event) {
			continue
		}
		matched = append(matched, rule.Name)
		if rule.SampleRate != nil {
			rate = *rule.SampleRate
			minRate = rule.MinSampleRate
		}
		if protected && !rule.ShapeProtected {
			if rule.Stop {
				break
			}
			continue
		}
		if shaped.Tags == nil && (len(rule.DropTags) > 0 || len(rule.RenameTags) > 0) {
			shaped.Tags = map[string]string{}
		}
		for _, key := range rule.DropTags {
			if _, ok := shaped.Tags[key]; ok {
				delete(shaped.Tags, key)
				dropped[key] = struct{}{}
			}
		}
		renameKeys := make([]string, 0, len(rule.RenameTags))
		for from := range rule.RenameTags {
			renameKeys = append(renameKeys, from)
		}
		sort.Strings(renameKeys)
		for _, from := range renameKeys {
			to := rule.RenameTags[from]
			value, ok := shaped.Tags[from]
			if !ok {
				continue
			}
			if _, exists := shaped.Tags[to]; !exists {
				shaped.Tags[to] = value
			}
			delete(shaped.Tags, from)
			renamed[from] = to
		}
		if rule.PayloadMaxBytes > 0 && len(shaped.Payload) > rule.PayloadMaxBytes && rule.PayloadAction == PayloadDrop {
			shaped.Payload = nil
			payloadDropped = true
			if shaped.Tags == nil {
				shaped.Tags = map[string]string{}
			}
			shaped.Tags["telemetryforge.payload_oversize"] = "true"
		}
		if rule.Stop {
			break
		}
	}

	effective := effectiveRate(config.Pressure, rate, minRate, queuePressure)
	keep := protected || deterministicKeep(event, config, effective)
	reason := "sampled by deterministic event hash"
	if protected {
		keep = true
		reason = "protected: " + protectionReason
	} else if effective >= 1 {
		reason = "sample rate keeps all matching events"
	} else if !keep {
		reason = "deterministic sample excluded event"
	}
	droppedList := make([]string, 0, len(dropped))
	for key := range dropped {
		droppedList = append(droppedList, key)
	}
	sort.Strings(droppedList)
	decision := Decision{TenantID: event.TenantID, EventID: event.ID, Source: event.Source, EventType: event.Type, ObservedAt: now, ConfigName: config.Name, ConfigVersion: config.Version, Rules: matched, Keep: keep, Protected: protected, ProtectionReason: protectionReason, BaseRate: rate, EffectiveRate: effective, QueuePressure: clamp(queuePressure, 0, 1), DroppedTags: droppedList, RenamedTags: renamed, PayloadDropped: payloadDropped, OriginalBytes: originalBytes, ShapedBytes: canonicalBytes(shaped), Reason: reason}
	return shaped, decision
}

func effectiveRate(pressure PressureConfig, base, min, current float64) float64 {
	base = clamp(base, 0, 1)
	min = clamp(min, 0, base)
	current = clamp(current, 0, 1)
	if !pressure.Enabled || current < pressure.HighWatermark {
		return base
	}
	factor := pressure.HighFactor
	if current >= pressure.CriticalWatermark {
		factor = pressure.CriticalFactor
	}
	result := base * factor
	if result < min {
		result = min
	}
	return clamp(result, 0, 1)
}

func deterministicKeep(event domain.Event, config Config, rate float64) bool {
	if rate <= 0 {
		return false
	}
	if rate >= 1 {
		return true
	}
	identity := event.ID
	if identity == "" {
		identity = event.TenantID + "|" + event.Source + "|" + event.Type + "|" + event.Timestamp.UTC().Format(time.RFC3339Nano) + "|" + event.CorrelationID
	}
	sum := sha256.Sum256([]byte(config.Name + "|" + config.Version + "|" + identity))
	value := binary.BigEndian.Uint64(sum[:8])
	fraction := float64(value) / float64(math.MaxUint64)
	return fraction < rate
}

func isProtected(protection Protection, event domain.Event) (bool, string) {
	actualSeverity := severity(event)
	if protection.Errors && (strings.Contains(strings.ToLower(event.Type), "error") || actualSeverity == "error" || actualSeverity == "critical" || actualSeverity == "fatal") {
		return true, "error/severe telemetry"
	}
	for _, candidate := range protection.Severities {
		if strings.EqualFold(strings.TrimSpace(candidate), actualSeverity) && actualSeverity != "" {
			return true, "protected severity " + actualSeverity
		}
	}
	for _, pattern := range protection.Types {
		if glob(pattern, event.Type) {
			return true, "protected event type " + pattern
		}
	}
	for key, pattern := range protection.Tags {
		if value, ok := event.Tags[key]; ok && glob(pattern, value) {
			return true, "protected tag " + key
		}
	}
	if protection.LatencyThresholdMS > 0 && event.Value != nil && strings.EqualFold(event.Unit, "ms") && (strings.Contains(strings.ToLower(event.Type), "latency") || strings.Contains(strings.ToLower(event.Type), "duration")) && *event.Value >= protection.LatencyThresholdMS {
		return true, "high-latency telemetry"
	}
	return false, ""
}

func cloneEvent(event domain.Event) domain.Event {
	result := event
	if event.Tags != nil {
		result.Tags = make(map[string]string, len(event.Tags))
		for k, v := range event.Tags {
			result.Tags[k] = v
		}
	}
	if event.Payload != nil {
		result.Payload = append(json.RawMessage(nil), event.Payload...)
	}
	return result
}
func canonicalBytes(event domain.Event) int { payload, _ := json.Marshal(event); return len(payload) }
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
