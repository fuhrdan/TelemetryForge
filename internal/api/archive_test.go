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
	incidentarchive "github.com/fuhrdan/TelemetryForge/internal/incidentarchive"
	"github.com/fuhrdan/TelemetryForge/internal/storage"
	"github.com/fuhrdan/TelemetryForge/internal/stream"
)

type archiveAPIReader struct {
	imports []incidentarchive.ImportProvenance
}

func (reader *archiveAPIReader) QueryEvents(context.Context, storage.Query) ([]domain.Event, error) {
	return nil, nil
}

func (reader *archiveAPIReader) ListIncidentArchiveImports(
	context.Context,
	int,
) ([]incidentarchive.ImportProvenance, error) {
	return reader.imports, nil
}

func TestArchiveImportHistoryRoute(t *testing.T) {
	reader := &archiveAPIReader{imports: []incidentarchive.ImportProvenance{{
		ArchiveID: "archive-1", SourceTenantID: "alpha",
		SourceIncidentID: "INC-1", ImportedIncidentID: "INC-1",
		FormatVersion: 1, TelemetryForgeVersion: "1.5.0",
		FileSHA256: "abc", Encrypted: true, ImportedAt: time.Now().UTC(),
	}}}
	server := NewServer(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		stream.NewMemoryPublisher(),
		Topics{Raw: "telemetry.raw", Metric: "telemetry.metrics"},
		reader,
	)

	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/archive-imports", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestArchiveImportHistoryLimitValidation(t *testing.T) {
	server := NewServer(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		stream.NewMemoryPublisher(),
		Topics{Raw: "telemetry.raw", Metric: "telemetry.metrics"},
		&archiveAPIReader{},
	)

	response := httptest.NewRecorder()
	server.ServeHTTP(
		response,
		httptest.NewRequest(http.MethodGet, "/api/v1/archive-imports?limit=999", nil),
	)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
