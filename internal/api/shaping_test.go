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
	"github.com/fuhrdan/TelemetryForge/internal/shaping"
	"github.com/fuhrdan/TelemetryForge/internal/storage"
	"github.com/fuhrdan/TelemetryForge/internal/stream"
)

type shapingReader struct {
	stats []shaping.Stat
	diffs []shaping.ShadowDiff
}

func (r *shapingReader) QueryEvents(context.Context, storage.Query) ([]domain.Event, error) {
	return nil, nil
}
func (r *shapingReader) ShapingSummary(context.Context, time.Time) (shaping.Summary, error) {
	return shaping.Summary{Observed: 10, Kept: 5}, nil
}
func (r *shapingReader) ListShapingStats(context.Context, time.Time, int) ([]shaping.Stat, error) {
	return r.stats, nil
}
func (r *shapingReader) ListShapingShadowDiffs(context.Context, int) ([]shaping.ShadowDiff, error) {
	return r.diffs, nil
}

func TestShapingStatsRoute(t *testing.T) {
	reader := &shapingReader{stats: []shaping.Stat{{ConfigName: "active", Observed: 10, Kept: 5}}}
	server := NewServer(slog.New(slog.NewTextHandler(io.Discard, nil)), stream.NewMemoryPublisher(), Topics{Raw: "raw", Metric: "metric"}, reader)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/shaping/stats?window=1h", nil)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
func TestShapingStatsRejectsHugeWindow(t *testing.T) {
	reader := &shapingReader{}
	server := NewServer(slog.New(slog.NewTextHandler(io.Discard, nil)), stream.NewMemoryPublisher(), Topics{Raw: "raw", Metric: "metric"}, reader)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/shaping/stats?window=1000h", nil)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", response.Code)
	}
}
