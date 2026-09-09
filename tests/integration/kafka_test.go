package integration

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/stream"
)

// TestKafkaPublish verifies the real producer path when an integration broker
// is explicitly supplied. Unit test runs skip this test so ordinary development
// does not require Docker or a locally installed Kafka broker.
func TestKafkaPublish(t *testing.T) {
	brokersValue := strings.TrimSpace(os.Getenv("TELEMETRYFORGE_INTEGRATION_KAFKA_BROKERS"))
	if brokersValue == "" {
		t.Skip("set TELEMETRYFORGE_INTEGRATION_KAFKA_BROKERS to run Kafka integration tests")
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	publisher, err := stream.NewKafkaPublisher(stream.KafkaConfig{
		Brokers:  strings.Split(brokersValue, ","),
		ClientID: "telemetryforge-integration-test",
	}, logger)
	if err != nil {
		t.Fatalf("create Kafka publisher: %v", err)
	}
	defer publisher.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := publisher.Ready(ctx); err != nil {
		t.Fatalf("Kafka readiness failed: %v", err)
	}

	event := domain.Event{
		ID:            "integration-test-event",
		Source:        "integration-test",
		Type:          "test.event",
		Timestamp:     time.Now().UTC(),
		SchemaVersion: "1.0",
	}

	if err := publisher.Publish(ctx, "telemetry.raw", event); err != nil {
		t.Fatalf("publish integration event: %v", err)
	}
}
