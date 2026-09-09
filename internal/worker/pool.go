package worker

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/reliability"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// Job represents one Kafka record after decoding.
//
// Failure receives exhausted/permanent failures. It should durably record or
// publish the dead-letter item. Ack is called only after successful processing
// or successful dead-letter handling.
type Job struct {
	Context context.Context
	Event   domain.Event
	Ack     func(context.Context) error
	Nack    func(error)
	Failure func(context.Context, domain.Event, error, int) error

	// Done is called exactly once after the worker has finished all handling
	// for this job, including acknowledgement or terminal-failure routing.
	// Kafka consumers use this callback to know when a polled batch is safe to
	// release for a consumer-group rebalance.
	Done func()
}

// Observer receives bounded-worker operational metrics.
type Observer interface {
	QueueDepth(current, capacity int)
	JobCompleted(duration time.Duration, attempts int, outcome string)
	Retry(classification string)
}

// Pool is a bounded worker pool with bounded retry behavior.
//
// The jobs channel is intentionally bounded. Retry attempts also remain inside
// one worker slot, so a failing dependency cannot spawn unlimited goroutines or
// memory allocations.
type Pool struct {
	jobs        chan Job
	processor   Processor
	logger      *slog.Logger
	workers     int
	retryPolicy reliability.RetryPolicy
	observer    Observer
	wg          sync.WaitGroup
}

// NewPool creates a pool using the default retry policy.
func NewPool(workers int, queueCapacity int, processor Processor, logger *slog.Logger) (*Pool, error) {
	return NewPoolWithRetryAndObserver(workers, queueCapacity, processor, logger, reliability.DefaultRetryPolicy(), nil)
}

// NewPoolWithObserver creates a pool with default retry policy and metrics hooks.
func NewPoolWithObserver(workers int, queueCapacity int, processor Processor, logger *slog.Logger, observer Observer) (*Pool, error) {
	return NewPoolWithRetryAndObserver(workers, queueCapacity, processor, logger, reliability.DefaultRetryPolicy(), observer)
}

// NewPoolWithRetry creates a pool with an explicit retry policy.
func NewPoolWithRetry(workers int, queueCapacity int, processor Processor, logger *slog.Logger, retryPolicy reliability.RetryPolicy) (*Pool, error) {
	return NewPoolWithRetryAndObserver(workers, queueCapacity, processor, logger, retryPolicy, nil)
}

// NewPoolWithRetryAndObserver creates a pool with explicit retry and observability.
func NewPoolWithRetryAndObserver(workers int, queueCapacity int, processor Processor, logger *slog.Logger, retryPolicy reliability.RetryPolicy, observer Observer) (*Pool, error) {
	if workers < 1 {
		return nil, errors.New("worker count must be at least 1")
	}
	if queueCapacity < 1 {
		return nil, errors.New("queue capacity must be at least 1")
	}
	if processor == nil {
		return nil, errors.New("processor is required")
	}
	if retryPolicy.MaxAttempts < 1 {
		return nil, errors.New("retry attempts must be at least 1")
	}

	return &Pool{
		jobs:        make(chan Job, queueCapacity),
		processor:   processor,
		logger:      logger,
		workers:     workers,
		retryPolicy: retryPolicy,
		observer:    observer,
	}, nil
}

// Start launches the configured workers.
func (pool *Pool) Start(ctx context.Context) {
	if pool.observer != nil {
		pool.observer.QueueDepth(len(pool.jobs), cap(pool.jobs))
	}
	for i := 0; i < pool.workers; i++ {
		pool.wg.Add(1)
		go pool.run(ctx, i+1)
	}
}

// Submit waits until queue capacity is available or the context is cancelled.
func (pool *Pool) Submit(ctx context.Context, job Job) error {
	select {
	case pool.jobs <- job:
		if pool.observer != nil {
			pool.observer.QueueDepth(len(pool.jobs), cap(pool.jobs))
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Close stops accepting new jobs and waits for queued work to finish.
func (pool *Pool) Close() {
	close(pool.jobs)
	pool.wg.Wait()
}

func (pool *Pool) run(ctx context.Context, workerID int) {
	defer pool.wg.Done()

	for job := range pool.jobs {
		if pool.observer != nil {
			pool.observer.QueueDepth(len(pool.jobs), cap(pool.jobs))
		}
		pool.handleJob(ctx, workerID, job)
	}
}

func (pool *Pool) handleJob(ctx context.Context, workerID int, job Job) {
	started := time.Now()
	attempts := 0
	outcome := "failed"
	jobCtx := ctx
	if job.Context != nil {
		jobCtx = job.Context
	}
	tracer := otel.Tracer("github.com/fuhrdan/TelemetryForge/worker")
	jobCtx, span := tracer.Start(jobCtx, "worker.process")
	span.SetAttributes(
		attribute.String("telemetry.source", job.Event.Source),
		attribute.String("telemetry.type", job.Event.Type),
	)
	defer func() {
		span.SetAttributes(attribute.String("telemetry.outcome", outcome), attribute.Int("telemetry.attempts", attempts))
		if outcome == "failed" || outcome == "ack_failed" {
			span.SetStatus(codes.Error, outcome)
		}
		span.End()
		if pool.observer != nil {
			pool.observer.JobCompleted(time.Since(started), attempts, outcome)
		}
	}()
	if job.Done != nil {
		defer job.Done()
	}

	processed, actualAttempts, err := pool.processWithRetry(jobCtx, workerID, job.Event)
	attempts = actualAttempts
	if err != nil {
		if job.Failure != nil {
			if failureErr := job.Failure(jobCtx, job.Event, err, attempts); failureErr == nil {
				if pool.ack(jobCtx, job, job.Event.ID, workerID) {
					outcome = "dlq"
					return
				}
				outcome = "ack_failed"
			} else {
				err = errors.Join(err, failureErr)
			}
		}

		if job.Nack != nil {
			job.Nack(err)
		}
		pool.logger.Error("telemetry processing failed",
			"worker", workerID,
			"event_id", job.Event.ID,
			"attempts", attempts,
			"classification", reliability.Classification(err),
			"error", err)
		return
	}

	if !pool.ack(jobCtx, job, processed.ID, workerID) {
		outcome = "ack_failed"
		return
	}
	outcome = "success"

	pool.logger.Debug("telemetry event processed",
		"worker", workerID, "event_id", processed.ID,
		"source", processed.Source, "attempts", attempts)
}

func (pool *Pool) processWithRetry(ctx context.Context, workerID int, event domain.Event) (domain.Event, int, error) {
	for attempt := 1; attempt <= pool.retryPolicy.MaxAttempts; attempt++ {
		processed, err := pool.processor.Process(ctx, event)
		if err == nil {
			return processed, attempt, nil
		}

		if !reliability.IsTransient(err) || attempt == pool.retryPolicy.MaxAttempts {
			return domain.Event{}, attempt, err
		}

		if pool.observer != nil {
			pool.observer.Retry(string(reliability.Classification(err)))
		}
		delay := pool.retryPolicy.Delay(attempt)
		pool.logger.Warn("transient processing failure; retrying",
			"worker", workerID,
			"event_id", event.ID,
			"attempt", attempt,
			"next_delay", delay.String(),
			"error", err)

		if err := pool.retryPolicy.Wait(ctx, attempt); err != nil {
			return domain.Event{}, attempt, err
		}
	}

	return domain.Event{}, pool.retryPolicy.MaxAttempts, errors.New("retry loop exhausted")
}

func (pool *Pool) ack(ctx context.Context, job Job, eventID string, workerID int) bool {
	if job.Ack == nil {
		return true
	}
	if err := job.Ack(ctx); err != nil {
		if job.Nack != nil {
			job.Nack(err)
		}
		pool.logger.Error("telemetry acknowledgement failed",
			"worker", workerID, "event_id", eventID, "error", err)
		return false
	}
	return true
}
