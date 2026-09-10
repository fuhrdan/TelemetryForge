package api

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/changeintel"
	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/security"
	"github.com/fuhrdan/TelemetryForge/internal/storage"
	"github.com/fuhrdan/TelemetryForge/internal/stream"
)

type changeAPIReader struct {
	analysis changeintel.Analysis
	markers  []changeintel.Marker
}

func (*changeAPIReader) QueryEvents(context.Context, storage.Query) ([]domain.Event, error) {
	return nil, nil
}
func (r *changeAPIReader) ListChangeMarkers(context.Context, int) ([]changeintel.Marker, error) {
	return r.markers, nil
}
func (*changeAPIReader) ChangeMarker(context.Context, string) (changeintel.Marker, error) {
	return changeintel.Marker{}, nil
}
func (*changeAPIReader) ChangesBetween(context.Context, time.Time, time.Time, int) ([]changeintel.Marker, error) {
	return nil, nil
}
func (r *changeAPIReader) AnalyzeChange(context.Context, string, time.Duration, time.Duration) (changeintel.Analysis, error) {
	return r.analysis, nil
}
func (r *changeAPIReader) ChangeAnalysis(context.Context, string) (changeintel.Analysis, bool, error) {
	return r.analysis, true, nil
}

func TestChangeIngestionBuildsProtectedCanonicalEvent(t *testing.T) {
	publisher := stream.NewMemoryPublisher()
	server := NewServer(slog.New(slog.NewTextHandler(io.Discard, nil)), publisher, Topics{Raw: "telemetry.raw", Metric: "telemetry.metrics"}, nil)
	body := `{"change_id":"DEP-42","source":"checkout-api","kind":"deployment","version":"4.12.7","git_sha":"abc123","summary":"release"}`
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/changes", bytes.NewBufferString(body)))
	if response.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	events := publisher.Events()
	if len(events) != 1 {
		t.Fatalf("events=%d", len(events))
	}
	event := events[0].Event
	if event.Type != "deployment.completed" || event.Tags["telemetryforge.change_id"] != "DEP-42" || event.Tags["telemetryforge.git_sha"] != "abc123" {
		t.Fatalf("event=%#v", event)
	}
}

func TestChangeAnalysisEndpoint(t *testing.T) {
	reader := &changeAPIReader{analysis: changeintel.Analysis{Assessment: "regression-associated"}}
	server := NewServer(slog.New(slog.NewTextHandler(io.Discard, nil)), stream.NewMemoryPublisher(), Topics{Raw: "telemetry.raw", Metric: "telemetry.metrics"}, reader)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/changes/DEP-42/analyze?before_seconds=900&after_seconds=900", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if !bytes.Contains(response.Body.Bytes(), []byte("regression-associated")) {
		t.Fatalf("body=%s", response.Body.String())
	}
}

func TestChangeIngestionRejectsUnknownStatus(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	publisher := stream.NewMemoryPublisher()
	server := NewServer(
		logger,
		publisher,
		Topics{Raw: "telemetry.raw", Metric: "telemetry.metrics"},
		&changeAPIReader{},
	)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/changes", strings.NewReader(`{
		"source":"checkout-api",
		"kind":"deployment",
		"status":"whatever"
	}`))
	request = request.WithContext(security.WithTenant(request.Context(), "alpha"))
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if len(publisher.Events()) != 0 {
		t.Fatal("invalid change status must not publish telemetry")
	}
}
