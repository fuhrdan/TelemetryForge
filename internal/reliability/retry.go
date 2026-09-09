package reliability

import (
	"context"
	"math/rand"
	"time"
)

// RetryPolicy bounds local retries before work is moved to the dead-letter path.
type RetryPolicy struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Jitter      float64
}

// DefaultRetryPolicy is intentionally conservative for worker-local retries.
// Kafka remains the durable backlog; local retry exists only to ride through
// short dependency hiccups without immediately producing DLQ noise.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts: 4,
		BaseDelay:   250 * time.Millisecond,
		MaxDelay:    5 * time.Second,
		Jitter:      0.20,
	}
}

// Delay returns exponential backoff capped at MaxDelay, with symmetric jitter.
func (policy RetryPolicy) Delay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := policy.BaseDelay << (attempt - 1)
	if delay > policy.MaxDelay {
		delay = policy.MaxDelay
	}
	if policy.Jitter <= 0 {
		return delay
	}

	spread := float64(delay) * policy.Jitter
	offset := (rand.Float64()*2 - 1) * spread
	value := time.Duration(float64(delay) + offset)
	if value < 0 {
		return 0
	}
	return value
}

// Wait sleeps for an attempt's delay or returns early on cancellation.
func (policy RetryPolicy) Wait(ctx context.Context, attempt int) error {
	timer := time.NewTimer(policy.Delay(attempt))
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
