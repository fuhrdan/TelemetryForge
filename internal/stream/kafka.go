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
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
)

const defaultProduceTimeout = 5 * time.Second

// KafkaPublisher publishes canonical TelemetryForge envelopes to Kafka.
//
// Records are keyed by tenant + source. This preserves per-source ordering
// inside a tenant without making two tenants with the same source name share a
// partition key merely because their service names match.
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
	Security       KafkaSecurityConfig
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

	options := []kgo.Opt{
		kgo.SeedBrokers(config.Brokers...),
		kgo.ClientID(clientID),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.RecordPartitioner(kgo.StickyKeyPartitioner(nil)),
		kgo.ProducerBatchCompression(kgo.SnappyCompression()),
	}
	securityOptions, err := kafkaSecurityOptions(config.Security)
	if err != nil {
		return nil, err
	}
	options = append(options, securityOptions...)

	client, err := kgo.NewClient(options...)
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

	tracer := otel.Tracer("github.com/fuhrdan/TelemetryForge/kafka")
	spanContext, span := tracer.Start(ctx, "kafka.produce")
	span.SetAttributes(
		attribute.String("messaging.system", "kafka"),
		attribute.String("messaging.destination.name", topic),
	)
	defer span.End()

	publishContext, cancel := context.WithTimeout(spanContext, publisher.produceTimeout)
	defer cancel()

	record := &kgo.Record{
		Topic: topic,
		Key:   []byte(event.TenantID + "|" + event.Source),
		Value: payload,
		Headers: []kgo.RecordHeader{
			{Key: "telemetryforge-event-id", Value: []byte(event.ID)},
			{Key: "telemetryforge-tenant-id", Value: []byte(event.TenantID)},
			{Key: "telemetryforge-schema-version", Value: []byte(event.SchemaVersion)},
			{Key: "telemetryforge-correlation-id", Value: []byte(event.CorrelationID)},
		},
	}
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(spanContext, carrier)
	for key, value := range carrier {
		record.Headers = append(record.Headers, kgo.RecordHeader{Key: key, Value: []byte(value)})
	}

	result := publisher.client.ProduceSync(publishContext, record)
	if err := result.FirstErr(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Kafka publish failed")
		return fmt.Errorf("publish to topic %q: %w", topic, err)
	}
	if len(result) > 0 && result[0].Record != nil {
		span.SetAttributes(
			attribute.Int("messaging.kafka.destination.partition", int(result[0].Record.Partition)),
			attribute.Int64("messaging.kafka.message.offset", result[0].Record.Offset),
		)
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
