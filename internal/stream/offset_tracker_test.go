package stream

import (
	"context"
	"errors"
	"testing"

	"github.com/twmb/franz-go/pkg/kgo"
)

type fakeCommitter struct {
	commits []*kgo.Record
	err     error
}

func (committer *fakeCommitter) CommitRecords(_ context.Context, records ...*kgo.Record) error {
	if committer.err != nil {
		return committer.err
	}
	committer.commits = append(committer.commits, records...)
	return nil
}

func kafkaRecord(topic string, partition int32, offset int64) *kgo.Record {
	return &kgo.Record{Topic: topic, Partition: partition, Offset: offset}
}

func TestAckCoordinatorWaitsForEarlierRecord(t *testing.T) {
	committer := &fakeCommitter{}
	coordinator := newAckCoordinator(committer)
	first := kafkaRecord("telemetry.raw", 2, 10)
	second := kafkaRecord("telemetry.raw", 2, 11)

	if err := coordinator.Register(first); err != nil {
		t.Fatal(err)
	}
	if err := coordinator.Register(second); err != nil {
		t.Fatal(err)
	}

	if err := coordinator.Ack(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	if len(committer.commits) != 0 {
		t.Fatalf("committed %d records before earlier offset completed", len(committer.commits))
	}

	if err := coordinator.Ack(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	if len(committer.commits) != 1 || committer.commits[0].Offset != 11 {
		t.Fatalf("unexpected commit set: %#v", committer.commits)
	}
}

func TestAckCoordinatorPartitionsAdvanceIndependently(t *testing.T) {
	committer := &fakeCommitter{}
	coordinator := newAckCoordinator(committer)
	left := kafkaRecord("telemetry.raw", 0, 4)
	right := kafkaRecord("telemetry.raw", 1, 9)

	_ = coordinator.Register(left)
	_ = coordinator.Register(right)

	if err := coordinator.Ack(context.Background(), right); err != nil {
		t.Fatal(err)
	}
	if len(committer.commits) != 1 || committer.commits[0].Partition != 1 {
		t.Fatalf("partition 1 should commit independently: %#v", committer.commits)
	}
}

func TestAckCoordinatorRetainsCompletedPrefixAfterCommitFailure(t *testing.T) {
	committer := &fakeCommitter{err: errors.New("coordinator unavailable")}
	coordinator := newAckCoordinator(committer)
	first := kafkaRecord("telemetry.raw", 0, 20)
	second := kafkaRecord("telemetry.raw", 0, 21)

	_ = coordinator.Register(first)
	_ = coordinator.Register(second)

	if err := coordinator.Ack(context.Background(), first); err == nil {
		t.Fatal("expected commit failure")
	}

	committer.err = nil
	if err := coordinator.Ack(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	if len(committer.commits) != 1 || committer.commits[0].Offset != 21 {
		t.Fatalf("expected retry to commit through offset 21: %#v", committer.commits)
	}
}
