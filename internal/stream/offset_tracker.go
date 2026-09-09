package stream

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/twmb/franz-go/pkg/kgo"
)

type recordCommitter interface {
	CommitRecords(ctx context.Context, records ...*kgo.Record) error
}

type partitionKey struct {
	topic     string
	partition int32
}

type trackedOffset struct {
	record *kgo.Record
	done   bool
}

type partitionAckState struct {
	mu      sync.Mutex
	pending []*trackedOffset
}

// ackCoordinator prevents an out-of-order worker completion from committing
// past an earlier record in the same Kafka partition.
//
// Kafka commits represent a partition position, not an independent boolean for
// each record. If offset 11 is committed while offset 10 is still processing,
// a crash can cause offset 10 to be skipped after restart. The coordinator
// therefore commits only the highest record in the completed prefix of each
// partition's registered work.
type ackCoordinator struct {
	committer recordCommitter

	mu         sync.Mutex
	partitions map[partitionKey]*partitionAckState
}

func newAckCoordinator(committer recordCommitter) *ackCoordinator {
	return &ackCoordinator{
		committer:  committer,
		partitions: make(map[partitionKey]*partitionAckState),
	}
}

// Register records the source-partition processing order before work is handed
// to the concurrent worker pool.
func (coordinator *ackCoordinator) Register(record *kgo.Record) error {
	if record == nil {
		return errors.New("cannot register a nil Kafka record")
	}

	state := coordinator.state(record)
	state.mu.Lock()
	defer state.mu.Unlock()

	for _, existing := range state.pending {
		if existing.record.Offset == record.Offset {
			return nil
		}
	}

	state.pending = append(state.pending, &trackedOffset{record: record})
	return nil
}

// Ack marks one record complete and commits only when all earlier registered
// records in the same partition are also complete.
func (coordinator *ackCoordinator) Ack(ctx context.Context, record *kgo.Record) error {
	if record == nil {
		return errors.New("cannot acknowledge a nil Kafka record")
	}

	state := coordinator.state(record)
	state.mu.Lock()
	defer state.mu.Unlock()

	found := false
	for _, entry := range state.pending {
		if entry.record.Offset == record.Offset {
			entry.done = true
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf(
			"Kafka record was not registered before acknowledgement: topic=%s partition=%d offset=%d",
			record.Topic,
			record.Partition,
			record.Offset,
		)
	}

	completedPrefix := 0
	for completedPrefix < len(state.pending) && state.pending[completedPrefix].done {
		completedPrefix++
	}
	if completedPrefix == 0 {
		return nil
	}

	highest := state.pending[completedPrefix-1].record
	if err := coordinator.committer.CommitRecords(ctx, highest); err != nil {
		// Keep the completed entries in memory. A later acknowledgement for this
		// partition can retry a commit at an equal or higher safe position. If no
		// later record arrives, restart/rebalance replays the uncommitted work.
		return fmt.Errorf(
			"commit contiguous Kafka prefix topic=%s partition=%d through offset=%d: %w",
			highest.Topic,
			highest.Partition,
			highest.Offset,
			err,
		)
	}

	state.pending = state.pending[completedPrefix:]
	return nil
}

func (coordinator *ackCoordinator) state(record *kgo.Record) *partitionAckState {
	key := partitionKey{topic: record.Topic, partition: record.Partition}

	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()

	state := coordinator.partitions[key]
	if state == nil {
		state = &partitionAckState{}
		coordinator.partitions[key] = state
	}
	return state
}
