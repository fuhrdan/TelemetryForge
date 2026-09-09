package evidence

import (
	"strings"
	"testing"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
)

func TestGraphBuildsSupportingAndContradictingEvidenceWithoutCausalClaim(t *testing.T) {
	start := time.Date(2026, 9, 9, 10, 0, 0, 0, time.UTC)
	high := 1500.0
	normal := 120.0
	events := []domain.Event{
		{ID: "deploy", Source: "checkout", Type: "deployment.completed", Timestamp: start, Tags: map[string]string{"version": "2.0"}},
		{ID: "slow", Source: "checkout", Type: "request.duration", Timestamp: start.Add(10 * time.Second), Value: &high, Unit: "ms", CorrelationID: "c1"},
		{ID: "error", Source: "checkout", Type: "request.error", Timestamp: start.Add(20 * time.Second), Tags: map[string]string{"severity": "error"}, CorrelationID: "c1"},
		{ID: "recover", Source: "checkout", Type: "request.duration", Timestamp: start.Add(50 * time.Second), Value: &normal, Unit: "ms"},
	}

	graph := Build("tenant-a", "INC-1", events, nil, nil)

	var changeSupport bool
	var recoveryContradiction bool
	for _, edge := range graph.Edges {
		if edge.Relation == "change-precedes-error" && edge.Assessment == AssessmentSupporting {
			changeSupport = true
			if !strings.Contains(strings.ToLower(edge.Reason), "does not prove") {
				t.Fatal("change edge must explicitly reject causal certainty")
			}
		}
		if edge.Relation == "recovery-signal" && edge.Assessment == AssessmentContradicting {
			recoveryContradiction = true
		}
	}
	if !changeSupport {
		t.Fatal("expected supporting change/error temporal association")
	}
	if !recoveryContradiction {
		t.Fatal("expected contradictory recovery evidence")
	}

	if graph.Disclaimer == "" {
		t.Fatal("evidence graph requires an explicit non-causality disclaimer")
	}
}

func TestGraphCorrelatesSharedTraceAndCorrelation(t *testing.T) {
	start := time.Now().UTC()
	events := []domain.Event{
		{ID: "a", Source: "api", Type: "request", Timestamp: start, CorrelationID: "corr", Tags: map[string]string{"trace_id": "trace"}},
		{ID: "b", Source: "worker", Type: "job", Timestamp: start.Add(time.Second), CorrelationID: "corr", Tags: map[string]string{"trace_id": "trace"}},
	}
	graph := Build("tenant-a", "INC-2", events, nil, nil)

	relations := map[string]bool{}
	for _, edge := range graph.Edges {
		relations[edge.Relation] = true
	}
	if !relations["shared-correlation"] || !relations["shared-trace"] {
		t.Fatalf("relations=%v", relations)
	}
}

func TestGraphDoesNotExportCorrelationOrTraceIdentifiers(t *testing.T) {
	event := domain.Event{
		ID: "evt", Source: "checkout", Type: "request",
		Timestamp:     time.Now().UTC(),
		CorrelationID: "sensitive-correlation",
		Tags: map[string]string{
			"trace_id": "sensitive-trace",
			"version":  "1.2.3",
		},
	}
	graph := Build("tenant-a", "INC-privacy", []domain.Event{event}, nil, nil)

	for _, node := range graph.Nodes {
		for key, value := range node.Attributes {
			if key == "correlation_id" || key == "trace_id" ||
				value == "sensitive-correlation" || value == "sensitive-trace" {
				t.Fatalf("sensitive relationship identifier leaked through graph attributes: %#v", node.Attributes)
			}
		}
	}
}
