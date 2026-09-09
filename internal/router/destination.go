package router

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/stream"
)

// Sender delivers one canonical post-policy event to a destination.
type Sender interface {
	Send(context.Context, domain.Event) error
	Ready(context.Context) error
	Close()
}

// SenderFactoryConfig supplies cluster defaults that are intentionally kept
// outside routing JSON so credentials/security policy remain environment-owned.
type SenderFactoryConfig struct {
	KafkaBrokers  []string
	KafkaSecurity stream.KafkaSecurityConfig
}

// NewSender constructs one destination client.
func NewSender(destination Destination, config SenderFactoryConfig) (Sender, error) {
	destination = withDestinationDefaults(destination)
	switch destination.Type {
	case DestinationKafka:
		brokers := destination.Brokers
		if len(brokers) == 0 {
			brokers = config.KafkaBrokers
		}
		if len(brokers) == 0 {
			return nil, fmt.Errorf("Kafka destination %q has no brokers", destination.Name)
		}
		publisher, err := stream.NewKafkaPublisher(stream.KafkaConfig{
			Brokers:        brokers,
			ClientID:       "telemetryforge-router-" + destination.Name,
			ProduceTimeout: time.Duration(destination.TimeoutMS) * time.Millisecond,
			Security:       config.KafkaSecurity,
		}, slog.Default())
		if err != nil {
			return nil, err
		}
		return &kafkaSender{publisher: publisher, topic: destination.Topic}, nil
	case DestinationHTTP:
		return newHTTPSender(destination)
	default:
		return nil, fmt.Errorf("unsupported destination type %q", destination.Type)
	}
}

type kafkaSender struct {
	publisher stream.Publisher
	topic     string
}

func (sender *kafkaSender) Send(ctx context.Context, event domain.Event) error {
	return sender.publisher.Publish(ctx, sender.topic, event)
}
func (sender *kafkaSender) Ready(ctx context.Context) error { return sender.publisher.Ready(ctx) }
func (sender *kafkaSender) Close()                          { sender.publisher.Close() }

type httpSender struct {
	destination Destination
	client      *http.Client
}

func newHTTPSender(destination Destination) (*httpSender, error) {
	if envName := strings.TrimSpace(destination.BearerTokenEnv); envName != "" {
		if strings.TrimSpace(os.Getenv(envName)) == "" {
			return nil, fmt.Errorf("HTTP destination bearer token environment %q is empty", envName)
		}
	}
	for headerName, envName := range destination.HeaderEnv {
		if strings.TrimSpace(os.Getenv(strings.TrimSpace(envName))) == "" {
			return nil, fmt.Errorf("HTTP destination header %q environment %q is empty", headerName, envName)
		}
	}
	return &httpSender{
		destination: destination,
		client:      &http.Client{Timeout: time.Duration(destination.TimeoutMS) * time.Millisecond},
	}, nil
}

func (sender *httpSender) Send(ctx context.Context, event domain.Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode event: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, sender.destination.URL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create HTTP destination request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", event.ID)
	request.Header.Set("X-TelemetryForge-Event-ID", event.ID)
	request.Header.Set("X-TelemetryForge-Tenant-ID", event.TenantID)
	if err := sender.applyConfiguredHeaders(request); err != nil {
		return err
	}

	response, err := sender.client.Do(request)
	if err != nil {
		return fmt.Errorf("HTTP destination request: %w", err)
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64*1024))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("HTTP destination returned %s", response.Status)
	}
	return nil
}

func (sender *httpSender) applyConfiguredHeaders(request *http.Request) error {
	for key, value := range sender.destination.Headers {
		request.Header.Set(key, value)
	}
	for key, envName := range sender.destination.HeaderEnv {
		envName = strings.TrimSpace(envName)
		value := strings.TrimSpace(os.Getenv(envName))
		if value == "" {
			return fmt.Errorf("HTTP destination header %q environment %q is empty", key, envName)
		}
		request.Header.Set(key, value)
	}
	if envName := strings.TrimSpace(sender.destination.BearerTokenEnv); envName != "" {
		token := strings.TrimSpace(os.Getenv(envName))
		if token == "" {
			return fmt.Errorf("HTTP destination bearer token environment %q is empty", envName)
		}
		request.Header.Set("Authorization", "Bearer "+token)
	}
	return nil
}

func (sender *httpSender) Ready(ctx context.Context) error {
	healthURL := strings.TrimSpace(sender.destination.HealthURL)
	if healthURL == "" {
		return nil
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		return err
	}
	if err := sender.applyConfiguredHeaders(request); err != nil {
		return err
	}
	response, err := sender.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 8*1024))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("health endpoint returned %s", response.Status)
	}
	return nil
}
func (sender *httpSender) Close() {}
