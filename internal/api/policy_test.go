package api

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/policy"
	"github.com/fuhrdan/TelemetryForge/internal/storage"
	"github.com/fuhrdan/TelemetryForge/internal/stream"
)

type policyReader struct {
	findings []policy.Finding
	diffs    []policy.Diff
	states   []policy.DistributedState
	budgets  []policy.BudgetStatus
}

func (reader *policyReader) QueryEvents(_ context.Context, _ storage.Query) ([]domain.Event, error) {
	return nil, nil
}

func (reader *policyReader) ListCardinalityFindings(_ context.Context, _ int) ([]storage.CardinalityFinding, error) {
	return reader.findings, nil
}

func (reader *policyReader) ListPolicyDiffs(_ context.Context, _ int) ([]storage.PolicyDiff, error) {
	return reader.diffs, nil
}

func (reader *policyReader) ListDistributedCardinalityStates(_ context.Context, _ string, _ int) ([]policy.DistributedState, error) {
	return reader.states, nil
}

func (reader *policyReader) ListCardinalityBudgetStatus(_ context.Context, _ int) ([]policy.BudgetStatus, error) {
	return reader.budgets, nil
}

func TestCardinalityFindingsRoute(t *testing.T) {
	reader := &policyReader{
		findings: []policy.Finding{{
			PolicyName:    "active",
			PolicyVersion: "1",
			Mode:          "active",
			Source:        "checkout",
			EventType:     "request.duration",
			Dimension:     "request_id",
			Action:        policy.ActionDropTag,
		}},
	}
	server := NewServer(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		stream.NewMemoryPublisher(),
		Topics{Raw: "telemetry.raw", Metric: "telemetry.metrics"},
		reader,
	)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/cardinality/findings", nil)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, response.Code, response.Body.String())
	}
}

func TestPolicyDiffLimitValidation(t *testing.T) {
	reader := &policyReader{}
	server := NewServer(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		stream.NewMemoryPublisher(),
		Topics{Raw: "telemetry.raw", Metric: "telemetry.metrics"},
		reader,
	)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/policy/shadow-diffs?limit=9999", nil)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d: %s", http.StatusBadRequest, response.Code, response.Body.String())
	}
}

func TestDistributedCardinalityRoute(t *testing.T) {
	reader := &policyReader{states: []policy.DistributedState{{
		Mode: "active", Source: "checkout", EventType: "request.duration",
		Dimension: "request_id", ObservedUnique: 100, ProjectedUnique: 300,
	}}}
	server := NewServer(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		stream.NewMemoryPublisher(),
		Topics{Raw: "telemetry.raw", Metric: "telemetry.metrics"},
		reader,
	)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/cardinality/state?mode=active", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
}

func TestCardinalityBudgetRoute(t *testing.T) {
	reader := &policyReader{budgets: []policy.BudgetStatus{{
		PolicyName: "active", PolicyVersion: "1.2.0", Mode: "active",
		BudgetName: "tenant-hourly-series", SeriesLimit: 1000,
		ObservedUnique: 500, ProjectedUnique: 800, ConsumptionPercent: 80,
		Status: "warning",
	}}}
	server := NewServer(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		stream.NewMemoryPublisher(),
		Topics{Raw: "telemetry.raw", Metric: "telemetry.metrics"},
		reader,
	)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/cardinality/budgets", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
}
