// Package intelligence turns existing TelemetryForge evidence into bounded,
// deterministic investigation and recommendation summaries.
//
// It is deliberately not a root-cause oracle. The package never invents
// evidence, never calls an external model, and never mutates production policy.
// Every conclusion links back to captured graph relationships or recorded
// replay/cost artifacts.
package intelligence

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/costsim"
	"github.com/fuhrdan/TelemetryForge/internal/evidence"
	"github.com/fuhrdan/TelemetryForge/internal/replay"
)

const ProductVersion = "2.0.0"

// EdgeCitation is an exact Evidence Graph relationship cited by a finding.
type EdgeCitation struct {
	From       string `json:"from"`
	To         string `json:"to"`
	Relation   string `json:"relation"`
	Assessment string `json:"assessment"`
	Reason     string `json:"reason"`
}

// ArtifactReference cites a recorded analysis artifact rather than a graph edge.
type ArtifactReference struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
	Note string `json:"note"`
}

// Finding is one evidence-backed investigative statement.
type Finding struct {
	ID            string         `json:"id"`
	Statement     string         `json:"statement"`
	Status        string         `json:"status"`
	Confidence    string         `json:"confidence"`
	Supporting    []EdgeCitation `json:"supporting,omitempty"`
	Contradicting []EdgeCitation `json:"contradicting,omitempty"`
	CitedNodeIDs  []string       `json:"cited_node_ids,omitempty"`
	Notes         string         `json:"notes"`
}

// Recommendation is advisory only. RequiresHumanApproval is always true for
// any recommendation that could affect production configuration.
type Recommendation struct {
	ID                    string              `json:"id"`
	Kind                  string              `json:"kind"`
	Title                 string              `json:"title"`
	Rationale             string              `json:"rationale"`
	ExpectedEffects       map[string]float64  `json:"expected_effects,omitempty"`
	Evidence              []ArtifactReference `json:"evidence,omitempty"`
	RequiresHumanApproval bool                `json:"requires_human_approval"`
	LifecycleAction       string              `json:"lifecycle_action"`
	Safety                string              `json:"safety"`
}

// Investigation is the deterministic v2 incident synthesis result.
type Investigation struct {
	IncidentID      string           `json:"incident_id"`
	GeneratedAt     time.Time        `json:"generated_at"`
	Engine          string           `json:"engine"`
	Status          string           `json:"status"`
	Summary         string           `json:"summary"`
	Findings        []Finding        `json:"findings"`
	Recommendations []Recommendation `json:"recommendations,omitempty"`
	Evidence        EvidenceSummary  `json:"evidence"`
	Disclaimer      string           `json:"disclaimer"`
}

// EvidenceSummary lets callers see the limits of the synthesis at a glance.
type EvidenceSummary struct {
	GraphNodes           int `json:"graph_nodes"`
	GraphEdges           int `json:"graph_edges"`
	SupportingEdges      int `json:"supporting_edges"`
	ContradictingEdges   int `json:"contradicting_edges"`
	ReplayRuns           int `json:"replay_runs"`
	CostSimulations      int `json:"cost_simulations"`
	FindingsWithConflict int `json:"findings_with_conflict"`
}

// Comparison describes bounded similarity between two incident evidence graphs.
type Comparison struct {
	LeftIncidentID       string    `json:"left_incident_id"`
	RightIncidentID      string    `json:"right_incident_id"`
	GeneratedAt          time.Time `json:"generated_at"`
	Similarity           float64   `json:"similarity"`
	Status               string    `json:"status"`
	SharedSources        []string  `json:"shared_sources,omitempty"`
	SharedEventTypes     []string  `json:"shared_event_types,omitempty"`
	SharedHypotheses     []string  `json:"shared_hypotheses,omitempty"`
	SharedChangeVersions []string  `json:"shared_change_versions,omitempty"`
	LeftCitedNodeIDs     []string  `json:"left_cited_node_ids,omitempty"`
	RightCitedNodeIDs    []string  `json:"right_cited_node_ids,omitempty"`
	Notes                []string  `json:"notes"`
	Disclaimer           string    `json:"disclaimer"`
}

// Investigate creates a deterministic evidence-first incident synthesis.
func Investigate(graph evidence.Graph, runs []replay.Run, simulations []costsim.Result) Investigation {
	result := Investigation{
		IncidentID:  graph.IncidentID,
		GeneratedAt: time.Now().UTC(),
		Engine:      "deterministic-evidence-synthesis-v1",
		Status:      "insufficient_evidence",
		Evidence: EvidenceSummary{
			GraphNodes:         len(graph.Nodes),
			GraphEdges:         len(graph.Edges),
			SupportingEdges:    graph.Summary.SupportingEdges,
			ContradictingEdges: graph.Summary.ContradictingEdges,
			ReplayRuns:         countReplayRuns(graph.IncidentID, runs),
			CostSimulations:    countCostSimulations(graph.IncidentID, simulations),
		},
		Disclaimer: "Findings summarize captured evidence. Association, sequence, replay, and cost effects are not proof of root cause. Production changes require explicit human review and lifecycle promotion.",
	}

	for _, hypothesis := range graph.Hypotheses {
		finding := findingFromHypothesis(hypothesis, graph)
		// A v2 finding must cite captured evidence. Unknown/future graph
		// hypotheses are ignored until their evidence relationships have an
		// explicit source-controlled citation mapping.
		if len(finding.Supporting) == 0 && len(finding.Contradicting) == 0 {
			continue
		}
		if len(finding.Contradicting) > 0 && len(finding.Supporting) > 0 {
			result.Evidence.FindingsWithConflict++
		}
		result.Findings = append(result.Findings, finding)
	}

	sort.SliceStable(result.Findings, func(i, j int) bool {
		return findingScore(result.Findings[i]) > findingScore(result.Findings[j])
	})

	supported := 0
	mixed := 0
	for _, finding := range result.Findings {
		switch finding.Status {
		case "supported":
			supported++
		case "mixed":
			mixed++
		}
	}

	switch {
	case supported == 0 && mixed == 0:
		result.Status = "insufficient_evidence"
		result.Summary = "The captured evidence does not support a sufficiently strong built-in causal hypothesis. Review the cited timeline and collect additional evidence before changing production behavior."
	case mixed > 0:
		result.Status = "mixed_evidence"
		result.Summary = "The incident contains evidence-backed investigative leads, but at least one lead also has contradictory evidence. Treat the findings as hypotheses requiring operator review."
	default:
		result.Status = "evidence_supported"
		result.Summary = "The incident contains one or more evidence-supported investigative leads. TelemetryForge is reporting associations and observed sequence, not declaring root cause."
	}

	result.Recommendations = recommendations(graph.IncidentID, runs, simulations)
	return result
}

func findingFromHypothesis(hypothesis evidence.Hypothesis, graph evidence.Graph) Finding {
	relations := hypothesisRelations(hypothesis.ID)
	finding := Finding{
		ID:        hypothesis.ID,
		Statement: hypothesis.Statement,
		Status:    hypothesis.Status,
		Notes:     hypothesis.Notes,
	}
	nodeIDs := make(map[string]struct{})
	for _, edge := range graph.Edges {
		if !relations[edge.Relation] {
			continue
		}
		citation := EdgeCitation{
			From: edge.From, To: edge.To, Relation: edge.Relation,
			Assessment: edge.Assessment, Reason: edge.Reason,
		}
		switch edge.Assessment {
		case evidence.AssessmentSupporting:
			finding.Supporting = append(finding.Supporting, citation)
		case evidence.AssessmentContradicting:
			finding.Contradicting = append(finding.Contradicting, citation)
		default:
			continue
		}
		nodeIDs[edge.From] = struct{}{}
		nodeIDs[edge.To] = struct{}{}
	}
	for id := range nodeIDs {
		finding.CitedNodeIDs = append(finding.CitedNodeIDs, id)
	}
	sort.Strings(finding.CitedNodeIDs)
	support := len(finding.Supporting)
	contradiction := len(finding.Contradicting)
	finding.Confidence = confidence(support, contradiction)
	switch {
	case support == 0:
		finding.Status = "insufficient"
	case contradiction > 0:
		finding.Status = "mixed"
	default:
		finding.Status = "supported"
	}
	return finding
}

func hypothesisRelations(id string) map[string]bool {
	switch id {
	case "recent-change-associated":
		return map[string]bool{
			"change-precedes-error":            true,
			"structured-change-precedes-error": true,
			"recovery-after-rollback":          true,
		}
	case "latency-before-errors":
		return map[string]bool{"latency-precedes-error": true}
	case "sustained-degradation":
		return map[string]bool{
			"latency-precedes-error":  true,
			"recovery-signal":         true,
			"recovery-after-rollback": true,
		}
	default:
		return map[string]bool{}
	}
}

func confidence(support, contradiction int) string {
	if support == 0 {
		return "insufficient"
	}
	if contradiction > 0 {
		return "mixed"
	}
	switch {
	case support >= 4:
		return "high"
	case support >= 2:
		return "medium"
	default:
		return "low"
	}
}

func findingScore(finding Finding) int {
	score := len(finding.Supporting)*10 - len(finding.Contradicting)*4
	switch finding.Confidence {
	case "high":
		score += 3
	case "medium":
		score += 2
	case "low":
		score++
	}
	return score
}

func recommendations(incidentID string, runs []replay.Run, simulations []costsim.Result) []Recommendation {
	latestSimulation, simulationOK := latestCost(incidentID, simulations)
	latestReplay, replayOK := latestReplay(incidentID, runs)
	if !simulationOK || latestSimulation.ShadowVersion == "" || latestSimulation.Error != "" {
		return nil
	}

	activeBytes := float64(latestSimulation.Active.Bytes)
	shadowBytes := float64(latestSimulation.Shadow.Bytes)
	activeSeries := float64(latestSimulation.Active.Series)
	shadowSeries := float64(latestSimulation.Shadow.Series)
	if activeBytes <= 0 || activeSeries <= 0 {
		return nil
	}

	byteReduction := percentageReduction(activeBytes, shadowBytes)
	seriesReduction := percentageReduction(activeSeries, shadowSeries)
	if byteReduction < 5 && seriesReduction < 5 {
		return nil
	}

	rec := Recommendation{
		ID:    "evaluate-shadow-policy-" + latestSimulation.ID,
		Kind:  "policy_evaluation",
		Title: "Evaluate the recorded shadow policy for controlled promotion",
		Rationale: fmt.Sprintf(
			"The latest recorded cost simulation for this incident projects %.1f%% lower canonical bytes and %.1f%% lower sampled series under shadow policy %s@%s relative to the active-policy sample.",
			byteReduction, seriesReduction, latestSimulation.ShadowPolicy, latestSimulation.ShadowVersion,
		),
		ExpectedEffects: map[string]float64{
			"canonical_bytes_reduction_percent": byteReduction,
			"sample_series_reduction_percent":   seriesReduction,
		},
		Evidence: []ArtifactReference{{
			Kind: "cost-simulation", ID: latestSimulation.ID,
			Note: "Directional frozen-incident projection; not a vendor bill or capacity guarantee.",
		}},
		RequiresHumanApproval: true,
		LifecycleAction:       "shadow -> evidence review -> approval -> activation",
		Safety:                "Recommendation is advisory only. TelemetryForge will not mutate or promote production policy automatically.",
	}

	if replayOK && latestReplay.Status == "completed" && latestReplay.ShadowVersion != "" {
		rec.Evidence = append(rec.Evidence, ArtifactReference{
			Kind: "replay", ID: latestReplay.ID,
			Note: fmt.Sprintf("Replay completed over %d events with %d shadow differences and %d quarantined events.", latestReplay.EventCount, latestReplay.ShadowDiffCount, latestReplay.QuarantinedCount),
		})
		if latestReplay.QuarantinedCount > 0 {
			rec.Safety += " Replay contains quarantined events; review them before any promotion."
		}
	} else {
		rec.Safety += " No completed shadow replay was found; run Incident Replay before promotion."
	}
	return []Recommendation{rec}
}

func latestReplay(incidentID string, runs []replay.Run) (replay.Run, bool) {
	filtered := make([]replay.Run, 0)
	for _, run := range runs {
		if run.IncidentID == incidentID {
			filtered = append(filtered, run)
		}
	}
	if len(filtered) == 0 {
		return replay.Run{}, false
	}
	sort.SliceStable(filtered, func(i, j int) bool { return filtered[i].StartedAt.After(filtered[j].StartedAt) })
	return filtered[0], true
}

func latestCost(incidentID string, simulations []costsim.Result) (costsim.Result, bool) {
	filtered := make([]costsim.Result, 0)
	for _, simulation := range simulations {
		if simulation.IncidentID == incidentID {
			filtered = append(filtered, simulation)
		}
	}
	if len(filtered) == 0 {
		return costsim.Result{}, false
	}
	sort.SliceStable(filtered, func(i, j int) bool { return filtered[i].StartedAt.After(filtered[j].StartedAt) })
	return filtered[0], true
}

func countReplayRuns(incidentID string, runs []replay.Run) int {
	count := 0
	for _, run := range runs {
		if run.IncidentID == incidentID {
			count++
		}
	}
	return count
}
func countCostSimulations(incidentID string, items []costsim.Result) int {
	count := 0
	for _, item := range items {
		if item.IncidentID == incidentID {
			count++
		}
	}
	return count
}

func percentageReduction(before, after float64) float64 {
	if before <= 0 {
		return 0
	}
	value := (before - after) / before * 100
	if math.Abs(value) < 0.05 {
		return 0
	}
	return math.Round(value*10) / 10
}

// Compare evaluates reproducible similarity between two Evidence Graphs.
func Compare(left, right evidence.Graph) Comparison {
	result := Comparison{
		LeftIncidentID:  left.IncidentID,
		RightIncidentID: right.IncidentID,
		GeneratedAt:     time.Now().UTC(),
		Status:          "insufficient_evidence",
		Disclaimer:      "Similarity indicates overlapping captured evidence, not shared root cause.",
	}

	leftSources, leftTypes, leftHypotheses, leftChanges := graphFeatures(left)
	rightSources, rightTypes, rightHypotheses, rightChanges := graphFeatures(right)

	result.SharedSources = intersection(leftSources, rightSources)
	result.SharedEventTypes = intersection(leftTypes, rightTypes)
	result.SharedHypotheses = intersection(leftHypotheses, rightHypotheses)
	result.SharedChangeVersions = intersection(leftChanges, rightChanges)

	sourceScore := jaccard(leftSources, rightSources)
	typeScore := jaccard(leftTypes, rightTypes)
	hypothesisScore := jaccard(leftHypotheses, rightHypotheses)
	changeScore := jaccard(leftChanges, rightChanges)
	result.Similarity = round3(sourceScore*0.30 + typeScore*0.25 + hypothesisScore*0.35 + changeScore*0.10)

	switch {
	case len(left.Nodes) == 0 || len(right.Nodes) == 0:
		result.Notes = append(result.Notes, "One incident has no graph nodes; comparison is not meaningful.")
	case result.Similarity >= 0.70:
		result.Status = "strong_overlap"
	case result.Similarity >= 0.40:
		result.Status = "partial_overlap"
	case result.Similarity > 0:
		result.Status = "limited_overlap"
	default:
		result.Notes = append(result.Notes, "No shared evidence features were found by the built-in comparison model.")
	}

	result.LeftCitedNodeIDs = citedNodes(left, result.SharedSources, result.SharedEventTypes, result.SharedChangeVersions)
	result.RightCitedNodeIDs = citedNodes(right, result.SharedSources, result.SharedEventTypes, result.SharedChangeVersions)
	result.Notes = append(result.Notes,
		fmt.Sprintf("Similarity components: sources %.3f, event types %.3f, hypotheses %.3f, change versions %.3f.", sourceScore, typeScore, hypothesisScore, changeScore),
	)
	return result
}

func graphFeatures(graph evidence.Graph) (map[string]bool, map[string]bool, map[string]bool, map[string]bool) {
	sources, types, hypotheses, changes := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, node := range graph.Nodes {
		if node.Source != "" {
			sources[node.Source] = true
		}
		if node.EventType != "" {
			types[node.EventType] = true
		}
		if node.Kind == "change" {
			for _, key := range []string{"version", "telemetryforge.version", "git_sha", "telemetryforge.git_sha"} {
				if value := strings.TrimSpace(node.Attributes[key]); value != "" {
					changes[key+":"+value] = true
				}
			}
		}
	}
	for _, hypothesis := range graph.Hypotheses {
		if hypothesis.Status != "insufficient" {
			hypotheses[hypothesis.ID] = true
		}
	}
	return sources, types, hypotheses, changes
}

func intersection(left, right map[string]bool) []string {
	result := make([]string, 0)
	for value := range left {
		if right[value] {
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

func jaccard(left, right map[string]bool) float64 {
	union := 0
	shared := 0
	seen := map[string]bool{}
	for value := range left {
		seen[value] = true
	}
	for value := range right {
		seen[value] = true
	}
	union = len(seen)
	if union == 0 {
		return 0
	}
	for value := range left {
		if right[value] {
			shared++
		}
	}
	return float64(shared) / float64(union)
}

func citedNodes(graph evidence.Graph, sources, types, changes []string) []string {
	sourceSet, typeSet, changeSet := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, value := range sources {
		sourceSet[value] = true
	}
	for _, value := range types {
		typeSet[value] = true
	}
	for _, value := range changes {
		changeSet[value] = true
	}
	result := make([]string, 0)
	for _, node := range graph.Nodes {
		include := sourceSet[node.Source] || typeSet[node.EventType]
		if node.Kind == "change" {
			for _, key := range []string{"version", "telemetryforge.version", "git_sha", "telemetryforge.git_sha"} {
				if changeSet[key+":"+node.Attributes[key]] {
					include = true
				}
			}
		}
		if include {
			result = append(result, node.ID)
		}
		if len(result) >= 100 {
			break
		}
	}
	sort.Strings(result)
	return result
}

func round3(value float64) float64 { return math.Round(value*1000) / 1000 }
