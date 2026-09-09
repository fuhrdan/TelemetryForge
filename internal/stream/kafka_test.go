package stream

import (
	"log/slog"
	"testing"
)

func TestNewKafkaPublisherRequiresBroker(t *testing.T) {
	_, err := NewKafkaPublisher(KafkaConfig{}, slog.Default())
	if err == nil {
		t.Fatal("expected missing broker configuration to fail")
	}
}
