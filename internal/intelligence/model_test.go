package intelligence

import (
	"testing"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/costsim"
	"github.com/fuhrdan/TelemetryForge/internal/evidence"
	"github.com/fuhrdan/TelemetryForge/internal/replay"
)

func TestInvestigateReturnsInsufficientEvidenceExplicitly(t *testing.T) {
	graph := evidence.Graph{IncidentID: "INC-1", Nodes: []evidence.Node{{ID: "incident:INC-1"}}, Hypotheses: []evidence.Hypothesis{{ID: "insufficient-evidence", Status: "insufficient"}}}
	result := Investigate(graph, nil, nil)
	if result.Status != "insufficient_evidence" {
		t.Fatalf("status=%q", result.Status)
	}
	if result.Summary == "" {
		t.Fatal("expected explicit summary")
	}
}

func TestInvestigateCitesSupportingAndContradictingEdges(t *testing.T) {
	graph := evidence.Graph{
		IncidentID: "INC-2",
		Nodes:      []evidence.Node{{ID: "change:c1"}, {ID: "event:e1"}, {ID: "event:r1"}},
		Edges: []evidence.Edge{
			{From: "change:c1", To: "event:e1", Relation: "structured-change-precedes-error", Assessment: evidence.AssessmentSupporting, Reason: "change before error"},
			{From: "change:c1", To: "event:r1", Relation: "recovery-after-rollback", Assessment: evidence.AssessmentContradicting, Reason: "recovery after rollback"},
		},
		Hypotheses: []evidence.Hypothesis{{ID: "recent-change-associated", Statement: "change associated", Status: "mixed", Supporting: 1, Contradicting: 1}},
	}
	result := Investigate(graph, nil, nil)
	if result.Status != "mixed_evidence" {
		t.Fatalf("status=%q", result.Status)
	}
	if len(result.Findings) != 1 || len(result.Findings[0].Supporting) != 1 || len(result.Findings[0].Contradicting) != 1 {
		t.Fatalf("finding=%#v", result.Findings)
	}
	if len(result.Findings[0].CitedNodeIDs) != 3 {
		t.Fatalf("nodes=%v", result.Findings[0].CitedNodeIDs)
	}
}

func TestRecommendationRequiresHumanApprovalAndCitesArtifacts(t *testing.T) {
	now := time.Now().UTC()
	graph := evidence.Graph{IncidentID: "INC-3", Nodes: []evidence.Node{{ID: "incident:INC-3"}}}
	simulation := costsim.Result{ID: "SIM-1", IncidentID: "INC-3", ShadowPolicy: "candidate", ShadowVersion: "2", StartedAt: now, Active: costsim.Profile{Bytes: 1000, Series: 100}, Shadow: costsim.Profile{Bytes: 500, Series: 60}}
	run := replay.Run{ID: "RPL-1", IncidentID: "INC-3", Status: "completed", ShadowPolicy: "candidate", ShadowVersion: "2", StartedAt: now, EventCount: 50}
	result := Investigate(graph, []replay.Run{run}, []costsim.Result{simulation})
	if len(result.Recommendations) != 1 {
		t.Fatalf("recommendations=%#v", result.Recommendations)
	}
	rec := result.Recommendations[0]
	if !rec.RequiresHumanApproval || len(rec.Evidence) != 2 {
		t.Fatalf("rec=%#v", rec)
	}
	if rec.ExpectedEffects["canonical_bytes_reduction_percent"] != 50 {
		t.Fatalf("effects=%#v", rec.ExpectedEffects)
	}
}

func TestCompareReportsDeterministicOverlap(t *testing.T) {
	left := evidence.Graph{IncidentID: "A", Nodes: []evidence.Node{{ID: "a1", Source: "api", EventType: "request.error"}}, Hypotheses: []evidence.Hypothesis{{ID: "latency-before-errors", Status: "supported"}}}
	right := evidence.Graph{IncidentID: "B", Nodes: []evidence.Node{{ID: "b1", Source: "api", EventType: "request.error"}}, Hypotheses: []evidence.Hypothesis{{ID: "latency-before-errors", Status: "supported"}}}
	result := Compare(left, right)
	if result.Status != "strong_overlap" || result.Similarity < .8 {
		t.Fatalf("comparison=%#v", result)
	}
	if len(result.LeftCitedNodeIDs) != 1 || len(result.RightCitedNodeIDs) != 1 {
		t.Fatalf("citations=%v/%v", result.LeftCitedNodeIDs, result.RightCitedNodeIDs)
	}
}

func TestInvestigateDoesNotSurfaceUncitedUnknownHypothesis(t *testing.T) {
	graph := evidence.Graph{
		IncidentID: "INC-UNKNOWN",
		Nodes:      []evidence.Node{{ID: "event:e1"}},
		Hypotheses: []evidence.Hypothesis{{
			ID: "future-hypothesis", Statement: "future statement", Status: "supported", Supporting: 4,
		}},
	}
	result := Investigate(graph, nil, nil)
	if result.Status != "insufficient_evidence" || len(result.Findings) != 0 {
		t.Fatalf("result=%#v", result)
	}
}
