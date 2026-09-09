package incident

import (
	"context"
	"testing"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
)

type memoryFreezer struct {
	count  int
	reason string
}

func (freezer *memoryFreezer) FreezeIncident(_ context.Context, _, _ string, _, _ time.Time) (int64, error) {
	freezer.count++
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
