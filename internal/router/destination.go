package router

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/fuhrdan/TelemetryForge/internal/connectors"
	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/stream"
)

type Sender interface {
	Kind() string
	Capabilities() connectors.Capabilities
	Send(context.Context, domain.Event) error
	Ready(context.Context) error
	Close()
}

type SenderFactoryConfig struct {
	KafkaBrokers  []string
	KafkaSecurity stream.KafkaSecurityConfig
	Logger        *slog.Logger
}

func NewSender(destination Destination, config SenderFactoryConfig) (Sender, error) {
	spec, err := ConnectorSpec(destination)
	if err != nil {
		return nil, err
	}
	connector, err := connectors.Build(spec, connectors.FactoryConfig{KafkaBrokers: config.KafkaBrokers, KafkaSecurity: config.KafkaSecurity, Logger: config.Logger})
	if err != nil {
		return nil, fmt.Errorf("%s connector: %w", spec.Kind, err)
	}
	return connector, nil
}

func newHTTPSender(destination Destination) (Sender, error) {
	destination.Type = DestinationHTTP
	return NewSender(destination, SenderFactoryConfig{})
}
