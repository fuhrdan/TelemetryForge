package api

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/storage"
	"github.com/fuhrdan/TelemetryForge/internal/stream"
)

type dashboardReader struct {
	summary   storage.Summary
	incidents []storage.Incident
	events    []domain.Event
}

func (reader *dashboardReader) QueryEvents(_ context.Context, _ storage.Query) ([]domain.Event, error) {
	return reader.events, nil
}

func (reader *dashboardReader) Summary(_ context.Context, _ time.Duration) (storage.Summary, error) {
	return reader.summary, nil
}

func (reader *dashboardReader) ListIncidents(_ context.Context, _ int) ([]storage.Incident, error) {
	return reader.incidents, nil
}

func (reader *dashboardReader) IncidentEvents(_ context.Context, _ string, _ int) ([]domain.Event, error) {
	return reader.events, nil
}

func (reader *dashboardReader) LiveEvents(_ context.Context, _ time.Time, _ string, _ int) ([]storage.LiveRecord, error) {
	records := make([]storage.LiveRecord, 0, len(reader.events))
	for _, event := range reader.events {
		records = append(records, storage.LiveRecord{Event: event, IngestedAt: time.Now().UTC()})
	}
	return records, nil
}

func dashboardTestServer(reader storage.Reader) *Server {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	publisher := stream.NewMemoryPublisher()
	return NewServer(
		logger,
		publisher,
		Topics{Raw: "telemetry.raw", Metric: "telemetry.metrics"},
		reader,
	)
}

func TestDashboardSummary(t *testing.T) {
	reader := &dashboardReader{
		summary: storage.Summary{
			Events:       100,
			ErrorCount:   2,
			ErrorRate:    0.02,
			P95LatencyMS: 180,
		},
	}
	server := dashboardTestServer(reader)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/summary?window=5m", nil)
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, response.Code, response.Body.String())
	}
	if body := response.Body.String(); body == "" {
		t.Fatal("expected dashboard summary JSON")
	}
}

func TestIncidentEventsRoute(t *testing.T) {
	reader := &dashboardReader{
		events: []domain.Event{{
			ID:            "evt-1",
			Source:        "checkout",
			Type:          "request.error",
			Timestamp:     time.Now().UTC(),
			SchemaVersion: "1.0",
		}},
	}
	server := dashboardTestServer(reader)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/incidents/INC-1/events", nil)
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, response.Code, response.Body.String())
	}
}
