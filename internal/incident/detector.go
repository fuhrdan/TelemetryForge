// Package incident contains automatic incident-detection primitives.
package incident

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
)

// Freezer is the storage behavior required to preserve an incident window.
type Freezer interface {
	FreezeIncident(ctx context.Context, incidentID, title string, from, to time.Time) (int64, error)
	AnnotateIncident(ctx context.Context, incidentID, reason string) error
}

// Config controls simple v0.6.0 automatic incident thresholds.
type Config struct {
	LatencyThresholdMS float64
	ErrorThreshold     int
	ErrorWindow        time.Duration
	FreezeLookback     time.Duration
	Cooldown           time.Duration
}

// DefaultConfig returns conservative demo defaults.
//
// These values are intentionally obvious and documented. v0.8.0 policy-as-code
// will make threshold behavior source-specific and version-controlled.
func DefaultConfig() Config {
	return Config{
		LatencyThresholdMS: 1000,
		ErrorThreshold:     5,
		ErrorWindow:        time.Minute,
		FreezeLookback:     15 * time.Minute,
		Cooldown:           10 * time.Minute,
	}
}

// Detector watches processed telemetry and freezes the rolling Flight Recorder
// when a simple threshold indicates a likely incident.
type Detector struct {
	freezer Freezer
	config  Config

	mu          sync.Mutex
	errors      map[string][]time.Time
	lastTrigger map[string]time.Time
}

// NewDetector creates an automatic incident detector.
func NewDetector(freezer Freezer, config Config) *Detector {
	return &Detector{
		freezer:     freezer,
		config:      config,
		errors:      make(map[string][]time.Time),
		lastTrigger: make(map[string]time.Time),
	}
}

// Observe evaluates one successfully persisted event.
//
// Detection happens after durable persistence so a transient database failure
// cannot generate an incident from telemetry that never reached the main store.
func (detector *Detector) Observe(ctx context.Context, event domain.Event) error {
	now := time.Now().UTC()

	if reason, ok := detector.latencyBreach(event); ok {
		return detector.trigger(ctx, event.Source, reason, now)
	}

	if isError(event) {
		if reason, ok := detector.recordError(event.Source, now); ok {
			return detector.trigger(ctx, event.Source, reason, now)
		}
	}

	return nil
}

func (detector *Detector) latencyBreach(event domain.Event) (string, bool) {
	if event.Value == nil || !strings.EqualFold(event.Unit, "ms") {
		return "", false
	}
	kind := strings.ToLower(event.Type)
	if !strings.Contains(kind, "latency") && !strings.Contains(kind, "duration") {
		return "", false
	}
	if *event.Value < detector.config.LatencyThresholdMS {
		return "", false
	}

	return fmt.Sprintf(
		"latency threshold breached: %.1fms >= %.1fms",
		*event.Value,
		detector.config.LatencyThresholdMS,
	), true
}

func (detector *Detector) recordError(source string, now time.Time) (string, bool) {
	detector.mu.Lock()
	defer detector.mu.Unlock()

	cutoff := now.Add(-detector.config.ErrorWindow)
	existing := detector.errors[source]
	kept := existing[:0]
	for _, timestamp := range existing {
		if timestamp.After(cutoff) {
			kept = append(kept, timestamp)
		}
	}
	kept = append(kept, now)
	detector.errors[source] = kept

	if len(kept) < detector.config.ErrorThreshold {
		return "", false
	}

	return fmt.Sprintf(
		"error threshold breached: %d errors within %s",
		len(kept),
		detector.config.ErrorWindow,
	), true
}

func (detector *Detector) trigger(ctx context.Context, source, reason string, now time.Time) error {
	key := source + "|" + reasonCategory(reason)

	detector.mu.Lock()
	if last := detector.lastTrigger[key]; !last.IsZero() && now.Sub(last) < detector.config.Cooldown {
		detector.mu.Unlock()
		return nil
	}
	detector.lastTrigger[key] = now
	detector.mu.Unlock()

	incidentID := fmt.Sprintf("AUTO-%d-%s", now.UnixMilli(), slug(source))
	title := fmt.Sprintf("Automatic incident: %s", source)
	from := now.Add(-detector.config.FreezeLookback)

	if _, err := detector.freezer.FreezeIncident(ctx, incidentID, title, from, now); err != nil {
		return fmt.Errorf("freeze automatic incident: %w", err)
	}
	if err := detector.freezer.AnnotateIncident(ctx, incidentID, reason); err != nil {
		return fmt.Errorf("annotate automatic incident: %w", err)
	}

	return nil
}

func isError(event domain.Event) bool {
	if strings.Contains(strings.ToLower(event.Type), "error") {
		return true
	}
	for _, key := range []string{"severity", "level"} {
		value := strings.ToLower(event.Tags[key])
		if value == "error" || value == "critical" || value == "fatal" {
			return true
		}
	}
	return false
}

func reasonCategory(reason string) string {
	if strings.HasPrefix(reason, "latency") {
		return "latency"
	}
	return "errors"
}

func slug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		default:
			if builder.Len() > 0 {
				builder.WriteByte('-')
			}
		}
	}
	result := strings.Trim(builder.String(), "-")
	if result == "" {
		return "unknown"
	}
	return result
}
