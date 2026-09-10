package connectors

import (
	"context"
	"fmt"
	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/stream"
	"time"
)

type kafkaConnector struct {
	spec      Spec
	publisher stream.Publisher
}

func newKafkaConnector(spec Spec, f FactoryConfig) (Connector, error) {
	b := spec.Brokers
	if len(b) == 0 {
		b = f.KafkaBrokers
	}
	if len(b) == 0 {
		return nil, fmt.Errorf("Kafka connector has no brokers")
	}
	p, err := stream.NewKafkaPublisher(stream.KafkaConfig{Brokers: b, ClientID: "telemetryforge-connector-" + sanitizeClientID(spec.Topic), ProduceTimeout: time.Duration(spec.TimeoutMS) * time.Millisecond, Security: f.KafkaSecurity}, f.Logger)
	if err != nil {
		return nil, err
	}
	return &kafkaConnector{spec: spec, publisher: p}, nil
}
func (c *kafkaConnector) Kind() string               { return KindKafka }
func (c *kafkaConnector) Capabilities() Capabilities { return capability(KindKafka) }
func (c *kafkaConnector) Send(ctx context.Context, e domain.Event) error {
	return c.publisher.Publish(ctx, c.spec.Topic, e)
}
func (c *kafkaConnector) Ready(ctx context.Context) error { return c.publisher.Ready(ctx) }
func (c *kafkaConnector) Close()                          { c.publisher.Close() }
func sanitizeClientID(v string) string {
	r := make([]rune, 0, len(v))
	for _, c := range v {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_':
			r = append(r, c)
		default:
			r = append(r, '-')
		}
	}
	if len(r) == 0 {
		return "destination"
	}
	return string(r)
}
