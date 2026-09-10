package router

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/connectors"
	"github.com/fuhrdan/TelemetryForge/internal/domain"
)

type dispatcherTestStore struct {
	delivered []string
	retried   []string
	dead      []string
	fallback  *Decision
}

func (store *dispatcherTestStore) ClaimRoutingDelivery(context.Context, string, time.Duration) (Delivery, bool, error) {
	return Delivery{}, false, nil
}
func (store *dispatcherTestStore) MarkRoutingDelivered(_ context.Context, delivery Delivery) error {
	store.delivered = append(store.delivered, delivery.Destination)
	return nil
}
func (store *dispatcherTestStore) MarkRoutingRetry(_ context.Context, delivery Delivery, _ string, _ time.Time) error {
	store.retried = append(store.retried, delivery.Destination)
	return nil
}
func (store *dispatcherTestStore) DeadLetterRoutingDelivery(_ context.Context, delivery Delivery, _ string, fallback *Decision) error {
	store.dead = append(store.dead, delivery.Destination)
	store.fallback = fallback
	return nil
}
func (*dispatcherTestStore) RecordRoutingDestinationHealth(context.Context, string, string, bool, string) error {
	return nil
}

type dispatcherTestSender struct{ err error }

func (dispatcherTestSender) Kind() string { return connectors.KindHTTPJSON }
func (dispatcherTestSender) Capabilities() connectors.Capabilities {
	return connectors.Capabilities{Kind: connectors.KindHTTPJSON, Protocol: "HTTP JSON"}
}
func (sender dispatcherTestSender) Send(context.Context, domain.Event) error { return sender.err }
func (dispatcherTestSender) Ready(context.Context) error                     { return nil }
func (dispatcherTestSender) Close()                                          {}

func TestDestinationFailureDoesNotPreventIndependentDelivery(t *testing.T) {
	store := &dispatcherTestStore{}
	dispatcher := &Dispatcher{
		config: Config{Name: "routing", Version: "1", Destinations: []Destination{
			{Name: "broken", Type: DestinationHTTP, Enabled: true, URL: "https://example.invalid"},
			{Name: "healthy", Type: DestinationHTTP, Enabled: true, URL: "https://example.invalid"},
		}},
		store: store,
		senders: map[string]Sender{
			"broken":  dispatcherTestSender{err: errors.New("backend down")},
			"healthy": dispatcherTestSender{},
		},
		logger: slog.Default(),
	}

	event := domain.Event{ID: "evt", TenantID: "tenant", Source: "checkout", Type: "request"}
	dispatcher.deliver(context.Background(), Delivery{
		TenantID: "tenant", EventID: event.ID, Destination: "broken", Event: event,
		Attempts: 1, MaxAttempts: 3, BaseDelayMS: 1, MaxDelayMS: 5,
	})
	dispatcher.deliver(context.Background(), Delivery{
		TenantID: "tenant", EventID: event.ID, Destination: "healthy", Event: event,
		Attempts: 1, MaxAttempts: 3, BaseDelayMS: 1, MaxDelayMS: 5,
	})

	if len(store.retried) != 1 || store.retried[0] != "broken" {
		t.Fatalf("retries=%v", store.retried)
	}
	if len(store.delivered) != 1 || store.delivered[0] != "healthy" {
		t.Fatalf("delivered=%v", store.delivered)
	}
}

func TestTerminalFailureCreatesConfiguredFallback(t *testing.T) {
	store := &dispatcherTestStore{}
	dispatcher := &Dispatcher{
		config: Config{Name: "routing", Version: "1", Destinations: []Destination{
			{Name: "security", Type: DestinationHTTP, Enabled: true, URL: "https://example.invalid", FailureFallback: "archive"},
			{Name: "archive", Type: DestinationKafka, Enabled: true, Topic: "telemetry.archive", MaxAttempts: 7, BaseDelayMS: 100, MaxDelayMS: 1000},
		}},
		store:   store,
		senders: map[string]Sender{"security": dispatcherTestSender{err: errors.New("backend down")}},
		logger:  slog.Default(),
	}

	event := domain.Event{ID: "evt", TenantID: "tenant"}
	dispatcher.deliver(context.Background(), Delivery{
		TenantID: "tenant", EventID: event.ID, Destination: "security", Event: event,
		Attempts: 3, MaxAttempts: 3, FailureFallback: "archive",
	})

	if len(store.dead) != 1 || store.dead[0] != "security" {
		t.Fatalf("dead letters=%v", store.dead)
	}
	if store.fallback == nil || store.fallback.Destination != "archive" || store.fallback.MaxAttempts != 7 {
		t.Fatalf("fallback=%#v", store.fallback)
	}
}

func TestPermanentConnectorFailureBypassesRetry(t *testing.T) {
	store := &dispatcherTestStore{}
	dispatcher := &Dispatcher{config: Config{Name: "routing", Version: "1", Destinations: []Destination{{Name: "rejecting", Type: DestinationHTTP, Enabled: true, URL: "https://example.invalid"}}}, store: store, senders: map[string]Sender{"rejecting": dispatcherTestSender{err: &connectors.DeliveryError{Err: errors.New("schema rejected"), Retryable: false, StatusCode: 400}}}, logger: slog.Default()}
	event := domain.Event{ID: "evt", TenantID: "tenant"}
	dispatcher.deliver(context.Background(), Delivery{TenantID: "tenant", EventID: event.ID, Destination: "rejecting", Event: event, Attempts: 1, MaxAttempts: 5})
	if len(store.retried) != 0 {
		t.Fatalf("permanent failure retried: %v", store.retried)
	}
	if len(store.dead) != 1 || store.dead[0] != "rejecting" {
		t.Fatalf("dead letters=%v", store.dead)
	}
}
