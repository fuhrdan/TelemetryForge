package connectors

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/stream"
)

type Connector interface {
	Kind() string
	Capabilities() Capabilities
	Send(context.Context, domain.Event) error
	Ready(context.Context) error
	Close()
}

type DeliveryError struct {
	Err        error
	Retryable  bool
	StatusCode int
}

func (e *DeliveryError) Error() string {
	if e == nil || e.Err == nil {
		return "connector delivery error"
	}
	return e.Err.Error()
}
func (e *DeliveryError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}
func Retryable(err error) bool {
	if err == nil {
		return false
	}
	var e *DeliveryError
	if errors.As(err, &e) {
		return e.Retryable
	}
	return true
}

type FactoryConfig struct {
	KafkaBrokers  []string
	KafkaSecurity stream.KafkaSecurityConfig
	Logger        *slog.Logger
	HTTPClient    *http.Client
}

func Build(spec Spec, factory FactoryConfig) (Connector, error) {
	spec = withDefaults(spec)
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	if factory.Logger == nil {
		factory.Logger = slog.Default()
	}
	switch spec.Kind {
	case KindKafka:
		return newKafkaConnector(spec, factory)
	case KindHTTPJSON:
		return newHTTPJSONConnector(spec, factory)
	case KindOTLPHTTP:
		return newOTLPHTTPConnector(spec, factory)
	case KindPrometheusRemoteWrite:
		return newPrometheusRemoteWriteConnector(spec, factory)
	case KindSplunkHEC:
		return newSplunkHECConnector(spec, factory)
	case KindDatadogLogs:
		return newDatadogLogsConnector(spec, factory)
	}
	return nil, fmt.Errorf("unsupported connector kind %q", spec.Kind)
}
func clientFor(spec Spec, factory FactoryConfig) *http.Client {
	if factory.HTTPClient != nil {
		return factory.HTTPClient
	}
	return &http.Client{Timeout: time.Duration(spec.TimeoutMS) * time.Millisecond}
}
