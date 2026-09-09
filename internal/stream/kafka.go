package stream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/twmb/franz-go/pkg/kgo"
)

const defaultProduceTimeout = 5 * time.Second

// KafkaPublisher publishes canonical TelemetryForge envelopes to Kafka.
//
// Records are keyed by source. Using a stable source key keeps telemetry from
// one producer ordered within a topic partition while still allowing Kafka to
// spread independent sources across partitions.
type KafkaPublisher struct {
	client         *kgo.Client
	logger         *slog.Logger
	produceTimeout time.Duration
}

// KafkaConfig contains the settings required by the gateway Kafka producer.
type KafkaConfig struct {
	Brokers        []string
	ClientID       string
	ProduceTimeout time.Duration
}

// NewKafkaPublisher creates a Kafka-backed Publisher.
//
// AllISR acknowledgements are requested so an HTTP 202 is returned only after
// Kafka has acknowledged the record according to the topic's in-sync replica
// policy. franz-go's idempotent producer behavior remains enabled by default.
func NewKafkaPublisher(config KafkaConfig, logger *slog.Logger) (*KafkaPublisher, error) {
	if len(config.Brokers) == 0 {
		return nil, errors.New("at least one Kafka broker is required")
	}

	clientID := strings.TrimSpace(config.ClientID)
	if clientID == "" {
		clientID = "telemetryforge-gateway"
	}

	produceTimeout := config.ProduceTimeout
	if produceTimeout <= 0 {
		produceTimeout = defaultProduceTimeout
	}

	client, err := kgo.NewClient(
		kgo.SeedBrokers(config.Brokers...),
		kgo.ClientID(clientID),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.RecordPartitioner(kgo.StickyKeyPartitioner(nil)),
		kgo.ProducerBatchCompression(kgo.SnappyCompression()),
	)
	if err != nil {
		return nil, fmt.Errorf("create Kafka client: %w", err)
	}

	return &KafkaPublisher{
		client:         client,
		logger:         logger,
		produceTimeout: produceTimeout,
	}, nil
}

// Publish serializes one canonical event and waits for Kafka acknowledgement.
func (publisher *KafkaPublisher) Publish(ctx context.Context, topic string, event domain.Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode event: %w", err)
	}

	publishContext, cancel := context.WithTimeout(ctx, publisher.produceTimeout)
	defer cancel()

	record := &kgo.Record{
		Topic: topic,
		Key:   []byte(event.Source),
		Value: payload,
		Headers: []kgo.RecordHeader{
			{Key: "telemetryforge-event-id", Value: []byte(event.ID)},
			{Key: "telemetryforge-schema-version", Value: []byte(event.SchemaVersion)},
			{Key: "telemetryforge-correlation-id", Value: []byte(event.CorrelationID)},
		},
	}

	result := publisher.client.ProduceSync(publishContext, record)
	if err := result.FirstErr(); err != nil {
		return fmt.Errorf("publish to topic %q: %w", topic, err)
	}

	publisher.logger.Debug(
		"telemetry event published",
		"event_id", event.ID,
		"topic", topic,
		"partition", result[0].Record.Partition,
		"offset", result[0].Record.Offset,
	)

	return nil
}

// Ready verifies that at least one configured Kafka broker is reachable.
func (publisher *KafkaPublisher) Ready(ctx context.Context) error {
	if err := publisher.client.Ping(ctx); err != nil {
		return fmt.Errorf("Kafka unavailable: %w", err)
	}

	return nil
}

// Close flushes pending producer work and closes broker connections.
func (publisher *KafkaPublisher) Close() {
	publisher.client.Close()
}
