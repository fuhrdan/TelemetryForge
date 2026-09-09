package api

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/schema"
	"github.com/fuhrdan/TelemetryForge/internal/storage"
	"github.com/fuhrdan/TelemetryForge/internal/stream"
)

type schemaAPIReader struct {
	entries []schema.RegistryEntry
	drifts  []schema.Drift
}

func (reader *schemaAPIReader) QueryEvents(context.Context, storage.Query) ([]domain.Event, error) {
	return nil, nil
}

func (reader *schemaAPIReader) ListSchemas(context.Context, int) ([]schema.RegistryEntry, error) {
	return reader.entries, nil
}

func (reader *schemaAPIReader) SchemaHistory(context.Context, string, string) ([]schema.RegistryEntry, error) {
	return reader.entries, nil
}

func (reader *schemaAPIReader) ListSchemaDrifts(context.Context, int) ([]schema.Drift, error) {
	return reader.drifts, nil
}

func (reader *schemaAPIReader) SchemaDiff(context.Context, string, string, string, string) (schema.VersionDiff, error) {
	return schema.VersionDiff{Compatibility: "compatible"}, nil
}

func TestSchemaRegistryRoute(t *testing.T) {
	reader := &schemaAPIReader{entries: []schema.RegistryEntry{{
		Source: "checkout", EventType: "request.duration", DeclaredVersion: "1.0", Health: "healthy",
	}}}
	server := NewServer(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		stream.NewMemoryPublisher(),
		Topics{Raw: "telemetry.raw", Metric: "telemetry.metrics"},
		reader,
	)

	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/schemas", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestSchemaHistoryRequiresIdentity(t *testing.T) {
	reader := &schemaAPIReader{}
	server := NewServer(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		stream.NewMemoryPublisher(),
		Topics{Raw: "telemetry.raw", Metric: "telemetry.metrics"},
		reader,
	)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/schema-history", nil))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", response.Code)
	}
}

func TestSchemaDiffRequiresVersions(t *testing.T) {
	reader := &schemaAPIReader{}
	server := NewServer(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		stream.NewMemoryPublisher(),
		Topics{Raw: "telemetry.raw", Metric: "telemetry.metrics"},
		reader,
	)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/schema-diff?source=a&type=b&from=1", nil))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", response.Code)
	}
}
