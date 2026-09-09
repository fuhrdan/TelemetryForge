package reliability

import (
	"errors"
	"testing"
	"time"
)

func TestClassification(t *testing.T) {
	base := errors.New("database unavailable")
	if !IsTransient(New(Transient, "persist", base)) {
		t.Fatal("expected transient error")
	}
	if IsTransient(New(Permanent, "validate", base)) {
		t.Fatal("permanent error must not be retryable")
	}
	if Classification(base) != Permanent {
		t.Fatal("unknown error should default to permanent")
	}
}

func TestBackoffCapsAtMaximum(t *testing.T) {
	policy := RetryPolicy{
		MaxAttempts: 10,
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    500 * time.Millisecond,
		Jitter:      0,
	}
	if got := policy.Delay(10); got != 500*time.Millisecond {
		t.Fatalf("delay=%s, want 500ms", got)
	}
}
