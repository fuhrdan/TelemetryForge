package api

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fuhrdan/TelemetryForge/internal/costsim"
	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/replay"
	"github.com/fuhrdan/TelemetryForge/internal/storage"
	"github.com/fuhrdan/TelemetryForge/internal/stream"
)

type replayHistoryReader struct {
	runs        []replay.Run
	simulations []costsim.Result
}

func (reader *replayHistoryReader) QueryEvents(_ context.Context, _ storage.Query) ([]domain.Event, error) {
	return nil, nil
}

func (reader *replayHistoryReader) ListReplayRuns(_ context.Context, _ int) ([]replay.Run, error) {
	return reader.runs, nil
}

func (reader *replayHistoryReader) ListCostSimulations(_ context.Context, _ int) ([]costsim.Result, error) {
	return reader.simulations, nil
}

func TestReplayHistoryRoute(t *testing.T) {
	reader := &replayHistoryReader{
		runs: []replay.Run{{ID: "RPL-1", IncidentID: "INC-1", Status: "completed"}},
	}
	server := NewServer(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		stream.NewMemoryPublisher(),
		Topics{Raw: "telemetry.raw", Metric: "telemetry.metrics"},
		reader,
	)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/replays", nil)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, response.Code, response.Body.String())
	}
}

func TestCostSimulationHistoryLimitValidation(t *testing.T) {
	reader := &replayHistoryReader{}
	server := NewServer(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		stream.NewMemoryPublisher(),
		Topics{Raw: "telemetry.raw", Metric: "telemetry.metrics"},
		reader,
	)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/cost-simulations?limit=999", nil)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d: %s", http.StatusBadRequest, response.Code, response.Body.String())
	}
}
