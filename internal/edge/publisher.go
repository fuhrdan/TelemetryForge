// Package edge connects the crash-safe WAL to TelemetryForge's existing stream publisher.
package edge

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/stream"
	"github.com/fuhrdan/TelemetryForge/internal/wal"
)

type Publisher struct {
	store      *wal.Store
	downstream stream.Publisher
	logger     *slog.Logger
	notify     chan struct{}
	stop       chan struct{}
	done       chan struct{}
	closeOnce  sync.Once
}

func NewPublisher(store *wal.Store, downstream stream.Publisher, logger *slog.Logger) *Publisher {
	publisher := &Publisher{store: store, downstream: downstream, logger: logger, notify: make(chan struct{}, 1), stop: make(chan struct{}), done: make(chan struct{})}
	go publisher.run()
	publisher.signal()
	return publisher
}

func (publisher *Publisher) Publish(ctx context.Context, topic string, event domain.Event) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	record, err := publisher.store.Append(topic, event)
	if err != nil {
		return err
	}
	publisher.logger.Debug("telemetry event durably accepted at edge", "event_id", event.ID, "edge_sequence", record.EdgeSequence, "source_sequence", record.SourceSequence, "topic", topic)
	publisher.signal()
	return nil
}

func (publisher *Publisher) Ready(_ context.Context) error { return publisher.store.Ready() }
func (publisher *Publisher) Stats() wal.Stats              { return publisher.store.Stats() }

func (publisher *Publisher) Close() {
	publisher.closeOnce.Do(func() {
		close(publisher.stop)
		<-publisher.done
		publisher.downstream.Close()
		if err := publisher.store.Close(); err != nil {
			publisher.logger.Error("edge WAL close failed", "error", err)
		}
	})
}

func (publisher *Publisher) run() {
	defer close(publisher.done)
	backoff := 100 * time.Millisecond
	const maxBackoff = 5 * time.Second
	for {
		select {
		case <-publisher.stop:
			return
		default:
		}
		records, err := publisher.store.Pending(1)
		if err != nil {
			if !errors.Is(err, wal.ErrClosed) {
				publisher.logger.Error("edge WAL replay failed", "error", err)
			}
			if !publisher.wait(backoff) {
				return
			}
			continue
		}
		if len(records) == 0 {
			backoff = 100 * time.Millisecond
			select {
			case <-publisher.stop:
				return
			case <-publisher.notify:
			case <-time.After(500 * time.Millisecond):
			}
			continue
		}
		record := records[0]
		deliveryContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err = publisher.downstream.Publish(deliveryContext, record.Topic, record.Event)
		cancel()
		if err != nil {
			publisher.logger.Warn("edge downstream delivery deferred", "edge_sequence", record.EdgeSequence, "event_id", record.Event.ID, "error", err)
			if !publisher.wait(backoff) {
				return
			}
			if backoff < maxBackoff {
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
			}
			continue
		}
		if err := publisher.store.Commit(record.EdgeSequence); err != nil {
			publisher.logger.Error("edge WAL checkpoint failed", "edge_sequence", record.EdgeSequence, "error", err)
			if !publisher.wait(backoff) {
				return
			}
			continue
		}
		backoff = 100 * time.Millisecond
	}
}

func (publisher *Publisher) signal() {
	select {
	case publisher.notify <- struct{}{}:
	default:
	}
}

func (publisher *Publisher) wait(duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-publisher.stop:
		return false
	case <-publisher.notify:
		return true
	case <-timer.C:
		return true
	}
}
