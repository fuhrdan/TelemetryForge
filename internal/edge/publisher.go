// Package edge connects the crash-safe WAL to TelemetryForge's existing stream publisher.
package edge

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/replication"
	"github.com/fuhrdan/TelemetryForge/internal/stream"
	"github.com/fuhrdan/TelemetryForge/internal/wal"
)

// Status combines local WAL state with the configured replication policy.
type Status struct {
	wal.Stats
	Replication replication.Status `json:"replication"`
}

type Publisher struct {
	store      *wal.Store
	downstream stream.Publisher
	replicator *replication.Manager
	logger     *slog.Logger
	notify     chan struct{}
	stop       chan struct{}
	done       chan struct{}
	closeOnce  sync.Once
}

// NewPublisher accepts an optional replication manager. The variadic form keeps
// v2.1 call sites source-compatible while allowing v2.2 to add quorum durability.
func NewPublisher(store *wal.Store, downstream stream.Publisher, logger *slog.Logger, replicators ...*replication.Manager) *Publisher {
	var replicator *replication.Manager
	if len(replicators) > 0 {
		replicator = replicators[0]
	}
	publisher := &Publisher{store: store, downstream: downstream, replicator: replicator, logger: logger, notify: make(chan struct{}, 1), stop: make(chan struct{}), done: make(chan struct{})}
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
	publisher.logger.Debug("telemetry event durably persisted at edge", "event_id", event.ID, "edge_sequence", record.EdgeSequence, "source_sequence", record.SourceSequence, "topic", topic)

	// Synchronous replication defines the v2.2 acceptance boundary. A failed
	// quorum attempt remains in the local WAL and the replay loop retries it.
	if publisher.replicator != nil {
		result, replicationErr := publisher.replicator.Replicate(ctx, record)
		if replicationErr != nil {
			publisher.logger.Warn("edge replication quorum not satisfied", "edge_sequence", record.EdgeSequence, "event_id", event.ID, "acks", result.AckCount, "quorum", result.Quorum, "mode", result.Mode, "error", replicationErr)
			publisher.signal()
			return replicationErr
		}
		publisher.logger.Debug("edge replication quorum satisfied", "edge_sequence", record.EdgeSequence, "event_id", event.ID, "acks", result.AckCount, "quorum", result.Quorum, "mode", result.Mode)
	}

	publisher.signal()
	return nil
}

func (publisher *Publisher) Ready(ctx context.Context) error {
	if err := publisher.store.Ready(); err != nil {
		return err
	}
	if publisher.replicator != nil {
		if err := publisher.replicator.Ready(ctx); err != nil {
			return err
		}
	}
	return publisher.downstream.Ready(ctx)
}

func (publisher *Publisher) Stats() Status {
	status := Status{Stats: publisher.store.Stats()}
	if publisher.replicator != nil {
		status.Replication = publisher.replicator.Status()
	} else {
		status.Replication = replication.Status{Mode: replication.ModeLocal, Quorum: 1, LastSatisfied: true, LastAckCount: 1}
	}
	return status
}

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
	var lastReleased uint64
	var lastReleaseAt time.Time
	releaseCommitted := func(force bool) {
		if publisher.replicator == nil {
			return
		}
		stats := publisher.store.Stats()
		if stats.CommittedSequence == 0 || stats.CommittedSequence <= lastReleased {
			return
		}
		if !force && !lastReleaseAt.IsZero() && time.Since(lastReleaseAt) < 5*time.Second {
			return
		}
		releaseContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := publisher.replicator.Release(releaseContext, stats.CommittedSequence)
		cancel()
		lastReleaseAt = time.Now()
		if err != nil {
			publisher.logger.Warn("edge replica release deferred", "committed_sequence", stats.CommittedSequence, "error", err)
			return
		}
		lastReleased = stats.CommittedSequence
	}
	releaseCommitted(true)
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
			releaseCommitted(false)
			select {
			case <-publisher.stop:
				return
			case <-publisher.notify:
			case <-time.After(500 * time.Millisecond):
			}
			continue
		}
		record := records[0]

		// Replication is always re-verified before downstream delivery. Peer
		// writes are idempotent, so restart/replay safely reconstructs quorum
		// without requiring a separate replication checkpoint file.
		if publisher.replicator != nil {
			replicationContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			result, replicationErr := publisher.replicator.Replicate(replicationContext, record)
			cancel()
			if replicationErr != nil {
				publisher.logger.Warn("edge replay waiting for replication quorum", "edge_sequence", record.EdgeSequence, "event_id", record.Event.ID, "acks", result.AckCount, "quorum", result.Quorum, "mode", result.Mode, "error", replicationErr)
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
		}

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
		if record.EdgeSequence-lastReleased >= 64 {
			releaseCommitted(true)
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
