package api

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/storage"
)

type queryReader struct {
	query storage.Query
	items []domain.Event
}

func (reader *queryReader) QueryEvents(_ context.Context, query storage.Query) ([]domain.Event, error) {
	reader.query = query
	return reader.items, nil
}

func TestParseStorageQuery(t *testing.T) {
	request := httptest.NewRequest("GET",
		"/api/v1/events?source=checkout&type=request.duration&limit=25&from=2026-09-09T00:00:00Z",
		nil)

	query, err := parseStorageQuery(request)
	if err != nil {
		t.Fatal(err)
	}
	if query.Source != "checkout" || query.Type != "request.duration" || query.Limit != 25 {
		t.Fatalf("unexpected query: %#v", query)
	}
}

func TestParseStorageQueryRejectsBadLimit(t *testing.T) {
	request := httptest.NewRequest("GET", "/api/v1/events?limit=5001", nil)
	if _, err := parseStorageQuery(request); err == nil {
		t.Fatal("expected invalid limit error")
	}
}
