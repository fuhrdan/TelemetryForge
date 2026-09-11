package api

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/intelligence"
	"github.com/fuhrdan/TelemetryForge/internal/storage"
	"github.com/fuhrdan/TelemetryForge/internal/stream"
)

type intelligenceAPIReader struct {
	events map[string][]domain.Event
	saved  intelligence.Investigation
}

func (*intelligenceAPIReader) QueryEvents(context.Context, storage.Query) ([]domain.Event, error) {
	return nil, nil
}
func (*intelligenceAPIReader) Summary(context.Context, time.Duration) (storage.Summary, error) {
	return storage.Summary{}, nil
}
func (*intelligenceAPIReader) ListIncidents(context.Context, int) ([]storage.Incident, error) {
	return nil, nil
}
func (reader *intelligenceAPIReader) IncidentEvents(_ context.Context, id string, _ int) ([]domain.Event, error) {
	return reader.events[id], nil
}
func (*intelligenceAPIReader) LiveEvents(context.Context, time.Time, string, int) ([]storage.LiveRecord, error) {
	return nil, nil
}
func (reader *intelligenceAPIReader) SaveInvestigation(_ context.Context, item intelligence.Investigation) error {
	reader.saved = item
	return nil
}
func (reader *intelligenceAPIReader) Investigation(context.Context, string) (intelligence.Investigation, bool, error) {
	return reader.saved, reader.saved.IncidentID != "", nil
}
func (reader *intelligenceAPIReader) ListInvestigations(context.Context, int) ([]intelligence.Investigation, error) {
	if reader.saved.IncidentID == "" {
		return nil, nil
	}
	return []intelligence.Investigation{reader.saved}, nil
}

func TestInvestigationReturnsExplicitInsufficientEvidenceAndPersistsSnapshot(t *testing.T) {
	now := time.Now().UTC()
	reader := &intelligenceAPIReader{events: map[string][]domain.Event{
		"INC-1": {{ID: "evt-1", TenantID: "default", Source: "api", Type: "request.ok", Timestamp: now, SchemaVersion: "1.0"}},
	}}
	server := NewServer(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		stream.NewMemoryPublisher(),
		Topics{Raw: "telemetry.raw", Metric: "telemetry.metrics"},
		reader,
	)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/incidents/INC-1/investigation", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"status":"insufficient_evidence"`) {
		t.Fatalf("body=%s", response.Body.String())
	}
	if reader.saved.IncidentID != "INC-1" {
		t.Fatalf("saved=%#v", reader.saved)
	}
}

func TestIncidentComparisonRejectsSameIncident(t *testing.T) {
	server := NewServer(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		stream.NewMemoryPublisher(),
		Topics{Raw: "telemetry.raw", Metric: "telemetry.metrics"},
		&intelligenceAPIReader{},
	)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/incidents/INC-1/compare?other=INC-1", nil))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
