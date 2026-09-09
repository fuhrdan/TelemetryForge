package api

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testServer() *Server {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewServer(logger)
}

func TestHealth(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	response := httptest.NewRecorder()

	testServer().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, response.Code)
	}
}

func TestAcceptEvent(t *testing.T) {
	body := `{
        "source":"checkout-api",
        "type":"request.duration",
        "timestamp":"2026-09-09T15:30:00Z",
        "schema_version":"1.0",
        "correlation_id":"order-123"
    }`

	request := httptest.NewRequest(http.MethodPost, "/api/v1/events", bytes.NewBufferString(body))
	response := httptest.NewRecorder()

	testServer().ServeHTTP(response, request)

	if response.Code != http.StatusAccepted {
		t.Fatalf("expected %d, got %d: %s", http.StatusAccepted, response.Code, response.Body.String())
	}

	if !strings.Contains(response.Body.String(), `"status":"accepted"`) {
		t.Fatalf("unexpected response: %s", response.Body.String())
	}
}

func TestRejectInvalidEvent(t *testing.T) {
	body := `{"source":"checkout-api","schema_version":"1.0"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/events", bytes.NewBufferString(body))
	response := httptest.NewRecorder()

	testServer().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, response.Code)
	}
}

func TestMetricRequiresValue(t *testing.T) {
	body := `{
        "source":"checkout-api",
        "type":"request.duration",
        "timestamp":"2026-09-09T15:30:00Z",
        "schema_version":"1.0"
    }`

	request := httptest.NewRequest(http.MethodPost, "/api/v1/metrics", bytes.NewBufferString(body))
	response := httptest.NewRecorder()

	testServer().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, response.Code)
	}
}
