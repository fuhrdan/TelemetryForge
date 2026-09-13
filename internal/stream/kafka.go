package stream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/fastpath"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
)

const defaultProduceTimeout = 5 * time.Second

var kafkaJSONBuffers = fastpath.NewBufferPool(1 << 20)

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
	results := publisher.PublishBatch(ctx, []BatchItem{{Topic: topic, Event: event}})
	if len(results) == 0 {
		return errors.New("Kafka batch publisher returned no result")
	}
	return results[0]
}

// PublishBatch submits an ordered batch to franz-go without a synchronous
// broker round trip per record. Pooled JSON buffers remain owned by the record
// until its callback fires, then are immediately returned to the bounded pool.
func (publisher *KafkaPublisher) PublishBatch(ctx context.Context, items []BatchItem) []error {
	results := make([]error, len(items))
	if len(items) == 0 {
		return results
	}

	tracer := otel.Tracer("github.com/fuhrdan/TelemetryForge/kafka")
	spanContext, span := tracer.Start(ctx, "kafka.produce.batch")
	span.SetAttributes(attribute.Int("messaging.batch.message_count", len(items)))
	defer span.End()

	publishContext, cancel := context.WithTimeout(spanContext, publisher.produceTimeout)
	defer cancel()

	var wait sync.WaitGroup
	for index, item := range items {
		buffer := kafkaJSONBuffers.Get()
		encoder := json.NewEncoder(buffer)
		if err := encoder.Encode(item.Event); err != nil {
			kafkaJSONBuffers.Put(buffer)
			results[index] = fmt.Errorf("encode event: %w", err)
			continue
		}
		payload := buffer.Bytes()
		if len(payload) > 0 && payload[len(payload)-1] == '\n' {
			payload = payload[:len(payload)-1]
		}
		record := newKafkaRecord(spanContext, item.Topic, item.Event, payload)
		wait.Add(1)
		idx := index
		ownedBuffer := buffer
		ownedTopic := item.Topic
		ownedEvent := item.Event
		publisher.client.Produce(publishContext, record, func(delivered *kgo.Record, err error) {
			defer wait.Done()
			defer kafkaJSONBuffers.Put(ownedBuffer)
			if err != nil {
				results[idx] = fmt.Errorf("publish to topic %q: %w", ownedTopic, err)
				return
			}
			publisher.logger.Debug("telemetry event published", "event_id", ownedEvent.ID, "topic", ownedTopic, "partition", delivered.Partition, "offset", delivered.Offset)
		})
	}
	wait.Wait()

	for _, err := range results {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "Kafka batch publish failed")
			break
		}
	}
	return results
}

func newKafkaRecord(ctx context.Context, topic string, event domain.Event, payload []byte) *kgo.Record {
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
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	for key, value := range carrier {
		record.Headers = append(record.Headers, kgo.RecordHeader{Key: key, Value: []byte(value)})
	}
	return record
}

// FastPathPoolStats reports process-local JSON buffer reuse for diagnostics.
func FastPathPoolStats() fastpath.PoolStats { return kafkaJSONBuffers.Stats() }

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
