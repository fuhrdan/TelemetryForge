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
	"github.com/fuhrdan/TelemetryForge/internal/worker"
	"github.com/twmb/franz-go/pkg/kgo"
)

// ConsumerConfig controls Kafka group consumption.
type ConsumerConfig struct {
	Brokers  []string
	ClientID string
	GroupID  string
	Topics   []string
}

// KafkaConsumer consumes telemetry using a Kafka consumer group.
//
// Auto-commit is disabled. A record is committed only after a worker finishes
// processing it successfully, which is the basis of TelemetryForge's
// at-least-once processing contract.
type KafkaConsumer struct {
	client *kgo.Client
	logger *slog.Logger
	lag    atomic.Int64
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

	client, err := kgo.NewClient(
		kgo.SeedBrokers(config.Brokers...),
		kgo.ClientID(clientID),
		kgo.ConsumerGroup(config.GroupID),
		kgo.ConsumeTopics(config.Topics...),
		kgo.DisableAutoCommit(),
		kgo.Balancers(kgo.CooperativeStickyBalancer()),
	)
	if err != nil {
		return nil, fmt.Errorf("create Kafka consumer: %w", err)
	}

	return &KafkaConsumer{client: client, logger: logger}, nil
}

// Run polls Kafka and submits decoded events to the bounded worker pool.
//
// Polling slows naturally when Pool.Submit blocks. This is deliberate: Kafka
// remains the durable queue while process memory stays bounded.
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
			var event domain.Event
			if err := json.Unmarshal(record.Value, &event); err != nil {
				consumer.logger.Error("invalid Kafka event",
					"topic", record.Topic, "partition", record.Partition,
					"offset", record.Offset, "error", err)
				continue
			}

			rec := record
			job := worker.Job{
				Event: event,
				Ack: func(ackCtx context.Context) error {
					return consumer.client.CommitRecords(ackCtx, rec)
				},
				Nack: func(err error) {
					consumer.logger.Warn("event left uncommitted for retry",
						"event_id", event.ID, "topic", rec.Topic,
						"partition", rec.Partition, "offset", rec.Offset, "error", err)
				},
			}

			if err := pool.Submit(ctx, job); err != nil {
				if ctx.Err() != nil {
					return nil
				}
				return fmt.Errorf("submit event to worker pool: %w", err)
			}

			// This is a local estimate for operational visibility in v0.3.0.
			// Broker-derived lag metrics arrive with the observability milestone.
			consumer.lag.Store(int64(fetches.NumRecords()))
		}
	}
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
