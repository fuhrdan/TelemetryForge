package worker

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/reliability"
)

type countingProcessor struct{ count atomic.Int32 }

func (processor *countingProcessor) Process(_ context.Context, event domain.Event) (domain.Event, error) {
	processor.count.Add(1)
	return event, nil
}

type failingProcessor struct{}

func (failingProcessor) Process(_ context.Context, event domain.Event) (domain.Event, error) {
	return event, errors.New("poison event")
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestPoolProcessesAndAcknowledgesJobs(t *testing.T) {
	processor := &countingProcessor{}
	pool, err := NewPool(2, 4, processor, testLogger())
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	pool.Start(ctx)

	var acknowledged atomic.Int32
	for i := 0; i < 3; i++ {
		err = pool.Submit(ctx, Job{
			Event: domain.Event{ID: "event", Source: "api", Type: "request"},
			Ack:   func(context.Context) error { acknowledged.Add(1); return nil },
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	pool.Close()

	if processor.count.Load() != 3 {
		t.Fatalf("processed %d events, want 3", processor.count.Load())
	}
	if acknowledged.Load() != 3 {
		t.Fatalf("acknowledged %d events, want 3", acknowledged.Load())
	}
}

func TestPoolNacksProcessingFailure(t *testing.T) {
	pool, err := NewPool(1, 1, failingProcessor{}, testLogger())
	if err != nil {
		t.Fatal(err)
	}
	pool.Start(context.Background())

	nacked := make(chan error, 1)
	if err := pool.Submit(context.Background(), Job{
		Event: domain.Event{ID: "bad", Source: "api", Type: "request"},
		Nack:  func(err error) { nacked <- err },
	}); err != nil {
		t.Fatal(err)
	}
	pool.Close()

	select {
	case err := <-nacked:
		if err == nil {
			t.Fatal("expected processing error")
		}
	case <-time.After(time.Second):
		t.Fatal("expected Nack callback")
	}
}

func TestNormalizerTrimsSourceAndType(t *testing.T) {
	event, err := (Normalizer{}).Process(context.Background(), domain.Event{
		Source: " checkout ", Type: " request.duration ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if event.Source != "checkout" || event.Type != "request.duration" {
		t.Fatalf("unexpected normalized event: %#v", event)
	}
}

type transientThenSuccessProcessor struct {
	attempts atomic.Int32
}

func (processor *transientThenSuccessProcessor) Process(_ context.Context, event domain.Event) (domain.Event, error) {
	attempt := processor.attempts.Add(1)
	if attempt < 3 {
		return domain.Event{}, reliability.New(reliability.Transient, "test", errors.New("temporary"))
	}
	return event, nil
}

func TestPoolRetriesTransientFailure(t *testing.T) {
	processor := &transientThenSuccessProcessor{}
	policy := reliability.RetryPolicy{
		MaxAttempts: 4,
		BaseDelay:   time.Millisecond,
		MaxDelay:    time.Millisecond,
		Jitter:      0,
	}
	pool, err := NewPoolWithRetry(1, 1, processor, testLogger(), policy)
	if err != nil {
		t.Fatal(err)
	}
	pool.Start(context.Background())

	acknowledged := make(chan struct{}, 1)
	if err := pool.Submit(context.Background(), Job{
		Event: domain.Event{ID: "retry-me", Source: "api", Type: "request"},
		Ack: func(context.Context) error {
			acknowledged <- struct{}{}
			return nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	pool.Close()

	if processor.attempts.Load() != 3 {
		t.Fatalf("attempts=%d, want 3", processor.attempts.Load())
	}
	select {
	case <-acknowledged:
	default:
		t.Fatal("expected successful acknowledgement after retry")
	}
}

func TestPoolRoutesPermanentFailureToFailureHandler(t *testing.T) {
	pool, err := NewPoolWithRetry(1, 1, failingProcessor{}, testLogger(), reliability.RetryPolicy{
		MaxAttempts: 4,
		BaseDelay:   time.Millisecond,
		MaxDelay:    time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	pool.Start(context.Background())

	failed := make(chan int, 1)
	acked := make(chan struct{}, 1)
	if err := pool.Submit(context.Background(), Job{
		Event: domain.Event{ID: "poison", Source: "api", Type: "request"},
		Failure: func(_ context.Context, _ domain.Event, _ error, attempts int) error {
			failed <- attempts
			return nil
		},
		Ack: func(context.Context) error {
			acked <- struct{}{}
			return nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	pool.Close()

	if attempts := <-failed; attempts != 1 {
		t.Fatalf("permanent failure attempted %d times, want 1", attempts)
	}
	select {
	case <-acked:
	default:
		t.Fatal("expected original record acknowledgement after DLQ handling")
	}
}

func TestPoolCallsDoneExactlyOnce(t *testing.T) {
	processor := &countingProcessor{}
	pool, err := NewPool(1, 1, processor, testLogger())
	if err != nil {
		t.Fatal(err)
	}
	pool.Start(context.Background())

	var done atomic.Int32
	if err := pool.Submit(context.Background(), Job{
		Event: domain.Event{ID: "done-once", Source: "api", Type: "request"},
		Ack:   func(context.Context) error { return nil },
		Done:  func() { done.Add(1) },
	}); err != nil {
		t.Fatal(err)
	}
	pool.Close()

	if got := done.Load(); got != 1 {
		t.Fatalf("Done called %d times, want exactly 1", got)
	}
}

func TestPoolCallsDoneAfterTerminalFailureHandling(t *testing.T) {
	pool, err := NewPoolWithRetry(1, 1, failingProcessor{}, testLogger(), reliability.RetryPolicy{
		MaxAttempts: 1,
		BaseDelay:   time.Millisecond,
		MaxDelay:    time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	pool.Start(context.Background())

	var failureHandled atomic.Bool
	var doneObservedFailure atomic.Bool

	if err := pool.Submit(context.Background(), Job{
		Event: domain.Event{ID: "terminal", Source: "api", Type: "request"},
		Failure: func(_ context.Context, _ domain.Event, _ error, _ int) error {
			failureHandled.Store(true)
			return nil
		},
		Ack: func(context.Context) error { return nil },
		Done: func() {
			doneObservedFailure.Store(failureHandled.Load())
		},
	}); err != nil {
		t.Fatal(err)
	}
	pool.Close()

	if !doneObservedFailure.Load() {
		t.Fatal("Done ran before terminal-failure handling completed")
	}
}
