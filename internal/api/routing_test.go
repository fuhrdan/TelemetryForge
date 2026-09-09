package api

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/router"
	"github.com/fuhrdan/TelemetryForge/internal/security"
	"github.com/fuhrdan/TelemetryForge/internal/storage"
	"github.com/fuhrdan/TelemetryForge/internal/stream"
)

type routingAPIReader struct {
	deliveries []router.Delivery
	diffs      []router.ShadowDiff
	letters    []router.DeadLetter
	health     []router.DestinationHealth
}

func (reader *routingAPIReader) QueryEvents(context.Context, storage.Query) ([]domain.Event, error) {
	return nil, nil
}
func (reader *routingAPIReader) ListRoutingDeliveries(context.Context, string, int) ([]router.Delivery, error) {
	return reader.deliveries, nil
}
func (reader *routingAPIReader) ListRoutingShadowDiffs(context.Context, int) ([]router.ShadowDiff, error) {
	return reader.diffs, nil
}
func (reader *routingAPIReader) ListRoutingDeadLetters(context.Context, string, int) ([]router.DeadLetter, error) {
	return reader.letters, nil
}
func (reader *routingAPIReader) ListRoutingDestinationHealth(context.Context, int) ([]router.DestinationHealth, error) {
	return reader.health, nil
}

func TestRoutingDestinationsRoute(t *testing.T) {
	reader := &routingAPIReader{health: []router.DestinationHealth{{
		Destination: "primary", State: "healthy",
	}}}
	server := NewServer(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		stream.NewMemoryPublisher(),
		Topics{Raw: "telemetry.raw", Metric: "telemetry.metrics"},
		reader,
	)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/routing/destinations", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestRoutingDeliveriesRejectInvalidStatus(t *testing.T) {
	reader := &routingAPIReader{}
	server := NewServer(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		stream.NewMemoryPublisher(),
		Topics{Raw: "telemetry.raw", Metric: "telemetry.metrics"},
		reader,
	)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/routing/deliveries?status=bogus", nil))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestRoutingDeliveryAPIUsesConfiguredRedaction(t *testing.T) {
	reader := &routingAPIReader{deliveries: []router.Delivery{{
		EventID: "evt-1", Destination: "primary",
		Event: domain.Event{ID: "evt-1", Tags: map[string]string{"email": "person@example.com"}},
	}}}
	server := NewServer(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		stream.NewMemoryPublisher(),
		Topics{Raw: "telemetry.raw", Metric: "telemetry.metrics"},
		reader,
	)
	server.SetRedactor(security.NewRedactor("email", false))
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/routing/deliveries", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "person@example.com") || !strings.Contains(response.Body.String(), "[REDACTED]") {
		t.Fatalf("routing delivery response did not apply redaction: %s", response.Body.String())
	}
}
