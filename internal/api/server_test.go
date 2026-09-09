package api

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
