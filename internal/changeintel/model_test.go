package changeintel

import (
	"testing"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
)

func TestMarkerFromDeploymentEvent(t *testing.T) {
	event := domain.Event{ID: "e1", TenantID: "alpha", Source: "checkout", Type: "deployment.completed", Timestamp: time.Now().UTC(), Tags: map[string]string{"deployment_id": "dep-7", "version": "4.12.7", "git_sha": "abc"}}
	marker, ok := MarkerFromEvent(event)
	if !ok {
		t.Fatal("deployment should be recognized")
	}
	if marker.ChangeID != "dep-7" || marker.Kind != KindDeployment || marker.Version != "4.12.7" {
		t.Fatalf("marker=%#v", marker)
	}
}

func TestAssessImpactRequiresEvidenceAndDetectsRegression(t *testing.T) {
	insufficient := AssessImpact("api", WindowStats{Events: 4}, WindowStats{Events: 20, Errors: 10, ErrorRate: .5})
	if insufficient.Assessment != AssessmentInsufficient {
		t.Fatalf("assessment=%s", insufficient.Assessment)
	}

	regression := AssessImpact("api",
		WindowStats{Events: 100, Errors: 1, ErrorRate: .01, P95LatencyMS: 200},
		WindowStats{Events: 100, Errors: 20, ErrorRate: .20, P95LatencyMS: 800})
	if regression.Assessment != AssessmentRegressed {
		t.Fatalf("assessment=%s", regression.Assessment)
	}
}

func TestFinalizeBlastRadius(t *testing.T) {
	analysis := Analysis{Change: Marker{Source: "checkout"}, SourceImpacts: []SourceImpact{
		{Source: "checkout", Assessment: AssessmentRegressed},
		{Source: "payments", Assessment: AssessmentRegressed},
		{Source: "search", Assessment: AssessmentUnchanged},
		{Source: "thin", Assessment: AssessmentInsufficient},
	}}
	analysis.Finalize()
	if analysis.RegressedSources != 2 || analysis.AssessableSources != 3 {
		t.Fatalf("analysis=%#v", analysis)
	}
	if analysis.BlastRadiusPercent < 66 || analysis.BlastRadiusPercent > 67 {
		t.Fatalf("blast=%f", analysis.BlastRadiusPercent)
	}
	if analysis.Assessment != "regression-associated" {
		t.Fatalf("assessment=%s", analysis.Assessment)
	}
}
