package incident

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
)

type memoryFreezer struct {
	count     int
	reason    string
	incident  string
	freezeErr error
}

func (freezer *memoryFreezer) FreezeIncident(_ context.Context, incidentID, _ string, _, _ time.Time) (int64, error) {
	freezer.count++
	freezer.incident = incidentID
	if freezer.freezeErr != nil {
		return 0, freezer.freezeErr
	}
	return 3, nil
}

func (freezer *memoryFreezer) AnnotateIncident(_ context.Context, _, reason string) error {
	freezer.reason = reason
	return nil
}

func TestLatencyThresholdTriggersIncident(t *testing.T) {
	freezer := &memoryFreezer{}
	config := DefaultConfig()
	config.Cooldown = 0
	detector := NewDetector(freezer, config)

	value := 1500.0
	err := detector.Observe(context.Background(), domain.Event{
		Source: "checkout",
		Type:   "request.duration",
		Value:  &value,
		Unit:   "ms",
	})
	if err != nil {
		t.Fatal(err)
	}
	if freezer.count != 1 {
		t.Fatalf("freeze count=%d, want 1", freezer.count)
	}
	if !strings.HasSuffix(freezer.incident, "-checkout-latency") {
		t.Fatalf("unexpected automatic incident id %q", freezer.incident)
	}
}

func TestErrorWindowTriggersAtThreshold(t *testing.T) {
	freezer := &memoryFreezer{}
	config := DefaultConfig()
	config.ErrorThreshold = 3
	config.Cooldown = 0
	detector := NewDetector(freezer, config)

	for i := 0; i < 3; i++ {
		if err := detector.Observe(context.Background(), domain.Event{
			Source: "payments",
			Type:   "payment.error",
		}); err != nil {
			t.Fatal(err)
		}
	}
	if freezer.count != 1 {
		t.Fatalf("freeze count=%d, want 1", freezer.count)
	}
}

func TestFailedFreezeReleasesCooldownReservation(t *testing.T) {
	freezer := &memoryFreezer{freezeErr: errors.New("storage unavailable")}
	config := DefaultConfig()
	config.Cooldown = time.Hour
	detector := NewDetector(freezer, config)

	value := 1500.0
	event := domain.Event{
		Source: "checkout",
		Type:   "request.duration",
		Value:  &value,
		Unit:   "ms",
	}

	if err := detector.Observe(context.Background(), event); err == nil {
		t.Fatal("expected first freeze to fail")
	}

	freezer.freezeErr = nil
	if err := detector.Observe(context.Background(), event); err != nil {
		t.Fatalf("second qualifying event should retry freeze immediately: %v", err)
	}

	if freezer.count != 2 {
		t.Fatalf("freeze attempts=%d, want 2", freezer.count)
	}
}

func TestErrorTrackingIsBoundedByConfiguredSourceLimit(t *testing.T) {
	freezer := &memoryFreezer{}
	config := DefaultConfig()
	config.ErrorThreshold = 100
	config.MaxTrackedSources = 3
	detector := NewDetector(freezer, config)

	for i := 0; i < 20; i++ {
		if err := detector.Observe(context.Background(), domain.Event{
			Source: strings.Repeat("x", i+1),
			Type:   "request.error",
		}); err != nil {
			t.Fatal(err)
		}
	}

	if got := len(detector.errors); got > config.MaxTrackedSources {
		t.Fatalf("tracked error sources=%d, max=%d", got, config.MaxTrackedSources)
	}
}

func TestTriggerTrackingIsBounded(t *testing.T) {
	freezer := &memoryFreezer{}
	config := DefaultConfig()
	config.Cooldown = time.Hour
	config.MaxTrackedSources = 2
	detector := NewDetector(freezer, config)

	value := 1500.0
	for i := 0; i < 20; i++ {
		if err := detector.Observe(context.Background(), domain.Event{
			Source: strings.Repeat("s", i+1),
			Type:   "request.duration",
			Value:  &value,
			Unit:   "ms",
		}); err != nil {
			t.Fatal(err)
		}
	}

	maxEntries := config.MaxTrackedSources * 2
	if got := len(detector.lastTrigger); got > maxEntries {
		t.Fatalf("tracked trigger keys=%d, max=%d", got, maxEntries)
	}
}

func TestSlugIsBounded(t *testing.T) {
	got := slug(strings.Repeat("Very Long Source Name ", 20))
	if len(got) > 64 {
		t.Fatalf("slug length=%d, want <=64", len(got))
	}
}
