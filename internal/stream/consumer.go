package stream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/reliability"
	"github.com/fuhrdan/TelemetryForge/internal/worker"
	"github.com/twmb/franz-go/pkg/kgo"
)

// ConsumerConfig controls Kafka group consumption and dead-letter routing.
type ConsumerConfig struct {
	Brokers  []string
	ClientID string
	GroupID  string
	Topics   []string
	DLQTopic string
}

// KafkaConsumer consumes telemetry using a Kafka consumer group.
//
// Auto-commit is disabled. A record is committed only after processing
// succeeds or after a terminal failure has been durably written to the DLQ.
type KafkaConsumer struct {
	client   *kgo.Client
	logger   *slog.Logger
	dlqTopic string
	lag      atomic.Int64
}

// NewKafkaConsumer creates a group consumer subscribed to the supplied topics.
func NewKafkaConsumer(config ConsumerConfig, logger *slog.Logger) (*KafkaConsumer, error) {
	if len(config.Brokers) == 0 {
		return nil, errors.New("at least one Kafka broker is required")
	}
	if strings.TrimSpace(config.GroupID) == "" {
		return nil, errors.New("Kafka group ID is required")
	}
	if len(config.Topics) == 0 {
		return nil, errors.New("at least one Kafka topic is required")
	}

	clientID := strings.TrimSpace(config.ClientID)
	if clientID == "" {
		clientID = "telemetryforge-worker"
	}
	dlqTopic := strings.TrimSpace(config.DLQTopic)
	if dlqTopic == "" {
		dlqTopic = "telemetry.dlq"
	}

	client, err := kgo.NewClient(
		kgo.SeedBrokers(config.Brokers...),
		kgo.ClientID(clientID),
		kgo.ConsumerGroup(config.GroupID),
		kgo.ConsumeTopics(config.Topics...),
		kgo.DisableAutoCommit(),
		kgo.Balancers(kgo.CooperativeStickyBalancer()),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.RecordPartitioner(kgo.StickyKeyPartitioner(nil)),
	)
	if err != nil {
		return nil, fmt.Errorf("create Kafka consumer: %w", err)
	}

	return &KafkaConsumer{client: client, logger: logger, dlqTopic: dlqTopic}, nil
}

// Run polls Kafka and submits decoded events to the bounded worker pool.
//
// Poison JSON is sent directly to the DLQ because no valid domain event can be
// created. Decoded events enter the worker retry policy; exhausted/permanent
// failures are routed to the same DLQ before the source offset is committed.
func (consumer *KafkaConsumer) Run(ctx context.Context, pool *worker.Pool) error {
	for {
		fetches := consumer.client.PollRecords(ctx, 100)
		if ctx.Err() != nil {
			return nil
		}
		if errs := fetches.Errors(); len(errs) > 0 {
			return fmt.Errorf("poll Kafka: %w", errs[0].Err)
		}

		for _, record := range fetches.Records() {
			rec := record
			var event domain.Event
			if err := json.Unmarshal(rec.Value, &event); err != nil {
				dead := domain.DeadLetter{
					RawPayload:    append([]byte(nil), rec.Value...),
					OriginalTopic: rec.Topic,
					Partition:     rec.Partition,
					Offset:        rec.Offset,
					FailureClass:  string(reliability.Permanent),
					Error:         "decode event: " + err.Error(),
					Attempts:      1,
					FailedAt:      time.Now().UTC(),
				}
				if dlqErr := consumer.publishDeadLetter(ctx, dead); dlqErr != nil {
					return fmt.Errorf("publish malformed record to DLQ: %w", dlqErr)
				}
				if err := consumer.client.CommitRecords(ctx, rec); err != nil {
					return fmt.Errorf("commit malformed DLQ record: %w", err)
				}
				continue
			}

			job := worker.Job{
				Event: event,
				Ack: func(ackCtx context.Context) error {
					return consumer.client.CommitRecords(ackCtx, rec)
				},
				Nack: func(err error) {
					consumer.logger.Warn("event remains uncommitted",
						"event_id", event.ID, "topic", rec.Topic,
						"partition", rec.Partition, "offset", rec.Offset, "error", err)
				},
				Failure: func(failureCtx context.Context, failed domain.Event, err error, attempts int) error {
					copyEvent := failed
					return consumer.publishDeadLetter(failureCtx, domain.DeadLetter{
						Event:         &copyEvent,
						OriginalTopic: rec.Topic,
						Partition:     rec.Partition,
						Offset:        rec.Offset,
						FailureClass:  string(reliability.Classification(err)),
						Error:         err.Error(),
						Attempts:      attempts,
						FailedAt:      time.Now().UTC(),
					})
				},
			}

			if err := pool.Submit(ctx, job); err != nil {
				if ctx.Err() != nil {
					return nil
				}
				return fmt.Errorf("submit event to worker pool: %w", err)
			}

			consumer.lag.Store(int64(fetches.NumRecords()))
		}
	}
}

func (consumer *KafkaConsumer) publishDeadLetter(ctx context.Context, dead domain.DeadLetter) error {
	payload, err := json.Marshal(dead)
	if err != nil {
		return fmt.Errorf("encode dead letter: %w", err)
	}

	key := []byte(dead.OriginalTopic)
	if dead.Event != nil && dead.Event.Source != "" {
		key = []byte(dead.Event.Source)
	}

	result := consumer.client.ProduceSync(ctx, &kgo.Record{
		Topic: consumer.dlqTopic,
		Key:   key,
		Value: payload,
		Headers: []kgo.RecordHeader{
			{Key: "telemetryforge-original-topic", Value: []byte(dead.OriginalTopic)},
			{Key: "telemetryforge-failure-class", Value: []byte(dead.FailureClass)},
		},
	})
	if err := result.FirstErr(); err != nil {
		return fmt.Errorf("publish DLQ record: %w", err)
	}

	consumer.logger.Error("telemetry moved to dead-letter queue",
		"original_topic", dead.OriginalTopic,
		"partition", dead.Partition,
		"offset", dead.Offset,
		"attempts", dead.Attempts,
		"classification", dead.FailureClass)
	return nil
}

// EstimatedLag returns the most recent local batch backlog estimate.
func (consumer *KafkaConsumer) EstimatedLag() int64 { return consumer.lag.Load() }

// Ready checks broker connectivity.
func (consumer *KafkaConsumer) Ready(ctx context.Context) error {
	readyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return consumer.client.Ping(readyCtx)
}

// Close leaves the consumer group and closes broker connections.
func (consumer *KafkaConsumer) Close() { consumer.client.Close() }
