package worker

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
)

// Job represents one Kafka record after decoding.
//
// Ack must be called only after processing succeeds. Nack records the failure
// without advancing the offset, preserving at-least-once processing semantics.
type Job struct {
	Event domain.Event
	Ack   func(context.Context) error
	Nack  func(error)
}

// Pool is a bounded worker pool.
//
// The jobs channel is intentionally bounded. An unbounded in-process queue
// would hide downstream slowness until the process exhausted memory. With a
// bounded queue, pressure propagates back to Kafka where durable backlog and
// consumer lag can be observed.
type Pool struct {
	jobs      chan Job
	processor Processor
	logger    *slog.Logger
	workers   int
	wg        sync.WaitGroup
}

// NewPool creates a worker pool with explicit concurrency and queue capacity.
func NewPool(workers int, queueCapacity int, processor Processor, logger *slog.Logger) (*Pool, error) {
	if workers < 1 {
		return nil, errors.New("worker count must be at least 1")
	}
	if queueCapacity < 1 {
		return nil, errors.New("queue capacity must be at least 1")
	}
	if processor == nil {
		return nil, errors.New("processor is required")
	}
	return &Pool{
		jobs: make(chan Job, queueCapacity), processor: processor,
		logger: logger, workers: workers,
	}, nil
}

// Start launches the configured workers.
func (pool *Pool) Start(ctx context.Context) {
	for i := 0; i < pool.workers; i++ {
		pool.wg.Add(1)
		go pool.run(ctx, i+1)
	}
}

// Submit waits until queue capacity is available or the context is cancelled.
//
// Waiting here is deliberate backpressure: the consumer slows down rather than
// allocating memory without bound.
func (pool *Pool) Submit(ctx context.Context, job Job) error {
	select {
	case pool.jobs <- job:
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
		processed, err := pool.processor.Process(ctx, job.Event)
		if err != nil {
			if job.Nack != nil {
				job.Nack(err)
			}
			pool.logger.Error("telemetry processing failed",
				"worker", workerID, "event_id", job.Event.ID, "error", err)
			continue
		}

		if job.Ack != nil {
			if err := job.Ack(ctx); err != nil {
				if job.Nack != nil {
					job.Nack(err)
				}
				pool.logger.Error("telemetry acknowledgement failed",
					"worker", workerID, "event_id", processed.ID, "error", err)
				continue
			}
		}

		pool.logger.Debug("telemetry event processed",
			"worker", workerID, "event_id", processed.ID, "source", processed.Source)
	}
}
