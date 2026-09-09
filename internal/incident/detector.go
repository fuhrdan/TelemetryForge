// Package incident contains automatic incident-detection primitives.
package incident

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/security"
)

const defaultMaxTrackedSources = 10000

// Freezer is the storage behavior required to preserve an incident window.
type Freezer interface {
	FreezeIncident(ctx context.Context, incidentID, title string, from, to time.Time) (int64, error)
	AnnotateIncident(ctx context.Context, incidentID, reason string) error
}

// Config controls simple automatic incident thresholds.
type Config struct {
	LatencyThresholdMS float64
	ErrorThreshold     int
	ErrorWindow        time.Duration
	FreezeLookback     time.Duration
	Cooldown           time.Duration

	// MaxTrackedSources bounds the in-memory error/cooldown state. Source names
	// arrive from telemetry and therefore cannot be trusted to have low
	// cardinality. Once the cap is reached, the oldest state is evicted.
	MaxTrackedSources int
}

// DefaultConfig returns conservative demo defaults.
//
// These values are intentionally obvious and documented. A later policy-as-code
// release will make threshold behavior source-specific and version-controlled.
func DefaultConfig() Config {
	return Config{
		LatencyThresholdMS: 1000,
		ErrorThreshold:     5,
		ErrorWindow:        time.Minute,
		FreezeLookback:     15 * time.Minute,
		Cooldown:           10 * time.Minute,
		MaxTrackedSources:  defaultMaxTrackedSources,
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
	defaults := DefaultConfig()
	if config.LatencyThresholdMS <= 0 {
		config.LatencyThresholdMS = defaults.LatencyThresholdMS
	}
	if config.ErrorThreshold < 1 {
		config.ErrorThreshold = defaults.ErrorThreshold
	}
	if config.ErrorWindow <= 0 {
		config.ErrorWindow = defaults.ErrorWindow
	}
	if config.FreezeLookback <= 0 {
		config.FreezeLookback = defaults.FreezeLookback
	}
	if config.MaxTrackedSources < 1 {
		config.MaxTrackedSources = defaults.MaxTrackedSources
	}

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

	tenantCtx := security.WithTenant(ctx, event.TenantID)
	sourceKey := event.TenantID + "|" + event.Source

	if reason, ok := detector.latencyBreach(event); ok {
		return detector.trigger(tenantCtx, sourceKey, event.Source, reason, now)
	}

	if isError(event) {
		if reason, ok := detector.recordError(sourceKey, now); ok {
			return detector.trigger(tenantCtx, sourceKey, event.Source, reason, now)
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

	if len(kept) == 0 {
		delete(detector.errors, source)
	}

	if _, exists := detector.errors[source]; !exists &&
		len(detector.errors) >= detector.config.MaxTrackedSources {
		detector.evictOldestErrorSourceLocked()
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

func (detector *Detector) trigger(
	ctx context.Context,
	sourceKey string,
	displaySource string,
	reason string,
	now time.Time,
) error {
	category := reasonCategory(reason)
	key := sourceKey + "|" + category

	detector.mu.Lock()
	detector.pruneExpiredTriggersLocked(now)

	if last := detector.lastTrigger[key]; !last.IsZero() &&
		detector.config.Cooldown > 0 &&
		now.Sub(last) < detector.config.Cooldown {
		detector.mu.Unlock()
		return nil
	}

	maxTriggerEntries := detector.config.MaxTrackedSources * 2
	if _, exists := detector.lastTrigger[key]; !exists &&
		len(detector.lastTrigger) >= maxTriggerEntries {
		detector.evictOldestTriggerLocked()
	}

	// Reserve before doing storage I/O so concurrent threshold breaches from
	// the same source/category cannot freeze duplicate incidents.
	detector.lastTrigger[key] = now
	detector.mu.Unlock()

	incidentID := fmt.Sprintf(
		"AUTO-%d-%s-%s",
		now.UnixMilli(),
		slug(displaySource),
		category,
	)
	title := fmt.Sprintf("Automatic incident: %s", displaySource)
	from := now.Add(-detector.config.FreezeLookback)

	if _, err := detector.freezer.FreezeIncident(ctx, incidentID, title, from, now); err != nil {
		// A failed freeze did not create useful incident evidence. Release the
		// reservation so the next qualifying event can retry immediately.
		detector.releaseTriggerReservation(key, now)
		return fmt.Errorf("freeze automatic incident: %w", err)
	}
	if err := detector.freezer.AnnotateIncident(ctx, incidentID, reason); err != nil {
		// The incident itself is already frozen, so keep the cooldown reservation
		// and report only the missing annotation.
		return fmt.Errorf("annotate automatic incident: %w", err)
	}

	return nil
}

func (detector *Detector) releaseTriggerReservation(key string, reservedAt time.Time) {
	detector.mu.Lock()
	defer detector.mu.Unlock()

	if current, ok := detector.lastTrigger[key]; ok && current.Equal(reservedAt) {
		delete(detector.lastTrigger, key)
	}
}

func (detector *Detector) pruneExpiredTriggersLocked(now time.Time) {
	if detector.config.Cooldown <= 0 {
		clear(detector.lastTrigger)
		return
	}

	cutoff := now.Add(-detector.config.Cooldown)
	for key, timestamp := range detector.lastTrigger {
		if timestamp.Before(cutoff) {
			delete(detector.lastTrigger, key)
		}
	}
}

func (detector *Detector) evictOldestErrorSourceLocked() {
	var oldestSource string
	var oldest time.Time

	for source, timestamps := range detector.errors {
		if len(timestamps) == 0 {
			delete(detector.errors, source)
			continue
		}

		last := timestamps[len(timestamps)-1]
		if oldestSource == "" || last.Before(oldest) {
			oldestSource = source
			oldest = last
		}
	}

	if oldestSource != "" {
		delete(detector.errors, oldestSource)
	}
}

func (detector *Detector) evictOldestTriggerLocked() {
	var oldestKey string
	var oldest time.Time

	for key, timestamp := range detector.lastTrigger {
		if oldestKey == "" || timestamp.Before(oldest) {
			oldestKey = key
			oldest = timestamp
		}
	}

	if oldestKey != "" {
		delete(detector.lastTrigger, oldestKey)
	}
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

		// Keep automatically generated primary keys/log fields manageable even
		// when an untrusted source name is extremely long.
		if builder.Len() >= 64 {
			break
		}
	}
	result := strings.Trim(builder.String(), "-")
	if result == "" {
		return "unknown"
	}
	return result
}
