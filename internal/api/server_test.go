package api

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/security"
	"github.com/fuhrdan/TelemetryForge/internal/storage"
	"github.com/fuhrdan/TelemetryForge/internal/stream"
)

func testServer() (*Server, *stream.MemoryPublisher) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	publisher := stream.NewMemoryPublisher()
	return NewServer(logger, publisher, Topics{Raw: "telemetry.raw", Metric: "telemetry.metrics"}), publisher
}

func TestHealth(t *testing.T) {
	server, _ := testServer()
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, response.Code)
	}
}

func TestReadyWhenPublisherAvailable(t *testing.T) {
	server, _ := testServer()
	request := httptest.NewRequest(http.MethodGet, "/ready", nil)
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, response.Code, response.Body.String())
	}
}

func TestNotReadyWhenKafkaUnavailable(t *testing.T) {
	server, publisher := testServer()
	publisher.SetError(errors.New("broker unavailable"))

	request := httptest.NewRequest(http.MethodGet, "/ready", nil)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected %d, got %d", http.StatusServiceUnavailable, response.Code)
	}
}

func TestAcceptEventPublishesRawTopic(t *testing.T) {
	body := `{
        "source":"checkout-api",
        "type":"request.duration",
        "timestamp":"2026-09-09T15:30:00Z",
        "schema_version":"1.0",
        "correlation_id":"order-123"
    }`

	server, publisher := testServer()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/events", bytes.NewBufferString(body))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusAccepted {
		t.Fatalf("expected %d, got %d: %s", http.StatusAccepted, response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"status":"accepted"`) {
		t.Fatalf("unexpected response: %s", response.Body.String())
	}

	events := publisher.Events()
	if len(events) != 1 {
		t.Fatalf("expected one publication, got %d", len(events))
	}
	if events[0].Topic != "telemetry.raw" {
		t.Fatalf("expected telemetry.raw, got %s", events[0].Topic)
	}
	if events[0].Event.ID == "" {
		t.Fatal("expected gateway-generated event ID")
	}
}

func TestAcceptMetricPublishesMetricTopic(t *testing.T) {
	body := `{
        "source":"checkout-api",
        "type":"request.duration",
        "timestamp":"2026-09-09T15:30:00Z",
        "schema_version":"1.0",
        "value":42.5,
        "unit":"ms"
    }`

	server, publisher := testServer()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/metrics", bytes.NewBufferString(body))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusAccepted {
		t.Fatalf("expected %d, got %d: %s", http.StatusAccepted, response.Code, response.Body.String())
	}
	events := publisher.Events()
	if len(events) != 1 || events[0].Topic != "telemetry.metrics" {
		t.Fatalf("metric was not published to telemetry.metrics: %#v", events)
	}
}

func TestPublishFailureReturnsServiceUnavailable(t *testing.T) {
	server, publisher := testServer()
	publisher.SetError(errors.New("broker unavailable"))

	body := `{
        "source":"checkout-api",
        "type":"request.duration",
        "timestamp":"2026-09-09T15:30:00Z",
        "schema_version":"1.0"
    }`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/events", bytes.NewBufferString(body))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected %d, got %d", http.StatusServiceUnavailable, response.Code)
	}
}

func TestRejectInvalidEvent(t *testing.T) {
	server, _ := testServer()
	body := `{"source":"checkout-api","schema_version":"1.0"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/events", bytes.NewBufferString(body))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, response.Code)
	}
}

func TestMetricRequiresValue(t *testing.T) {
	server, _ := testServer()
	body := `{
        "source":"checkout-api",
        "type":"request.duration",
        "timestamp":"2026-09-09T15:30:00Z",
        "schema_version":"1.0"
    }`

	request := httptest.NewRequest(http.MethodPost, "/api/v1/metrics", bytes.NewBufferString(body))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, response.Code)
	}
}

type readinessReader struct {
	err error
}

func (reader *readinessReader) QueryEvents(_ context.Context, _ storage.Query) ([]domain.Event, error) {
	return nil, nil
}

func (reader *readinessReader) Ready(_ context.Context) error {
	return reader.err
}

func TestNotReadyWhenDatabaseUnavailable(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	publisher := stream.NewMemoryPublisher()
	reader := &readinessReader{err: errors.New("database unavailable")}
	server := NewServer(
		logger,
		publisher,
		Topics{Raw: "telemetry.raw", Metric: "telemetry.metrics"},
		reader,
	)

	request := httptest.NewRequest(http.MethodGet, "/ready", nil)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected %d, got %d: %s", http.StatusServiceUnavailable, response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"dependency":"database"`) {
		t.Fatalf("expected database readiness failure, got %s", response.Body.String())
	}
}

func TestIngestRejectsClientSuppliedTenantID(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	publisher := stream.NewMemoryPublisher()
	server := NewServer(
		logger,
		publisher,
		Topics{Raw: "telemetry.raw", Metric: "telemetry.metrics"},
	)

	body := `{
	  "tenant_id":"other",
	  "source":"checkout",
	  "type":"request",
	  "timestamp":"2026-09-09T12:00:00Z",
	  "schema_version":"1.0"
	}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/events", strings.NewReader(body))
	request = request.WithContext(security.WithTenant(request.Context(), "alpha"))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected %d, got %d: %s", http.StatusForbidden, response.Code, response.Body.String())
	}
	if len(publisher.Events()) != 0 {
		t.Fatal("spoofed tenant event must not reach Kafka publisher")
	}
}
