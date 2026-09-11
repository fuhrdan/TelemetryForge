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
	"github.com/fuhrdan/TelemetryForge/internal/proof"
	"github.com/fuhrdan/TelemetryForge/internal/storage"
	"github.com/fuhrdan/TelemetryForge/internal/stream"
)

type proofAPIReader struct{ items []proof.Stored }

func (*proofAPIReader) QueryEvents(context.Context, storage.Query) ([]domain.Event, error) {
	return nil, nil
}
func (r *proofAPIReader) ListOperationalProofs(context.Context, int) ([]proof.Stored, error) {
	return r.items, nil
}

func TestProofHistoryRoute(t *testing.T) {
	now := time.Now().UTC()
	reader := &proofAPIReader{items: []proof.Stored{{Run: proof.Run{Format: proof.Format, FormatVersion: proof.Version, RunID: "run-1", GitCommit: "deadbeef", Scenario: "worker-failover", Status: "pass", StartedAt: now, CompletedAt: now.Add(time.Second), Assertions: []proof.Assertion{{Name: "worker-remained", Passed: true}}, Evidence: []proof.Evidence{{Kind: "log", Reference: "run.log"}}, Configuration: []proof.Fingerprint{{Name: "compose", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}}, ArtifactSHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", ArtifactBytes: 123, RecordedAt: now}}}
	server := NewServer(slog.New(slog.NewTextHandler(io.Discard, nil)), stream.NewMemoryPublisher(), Topics{Raw: "telemetry.raw", Metric: "telemetry.metrics"}, reader)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/proofs", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
