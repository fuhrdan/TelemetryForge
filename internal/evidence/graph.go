// Package evidence builds evidence-backed incident relationship graphs.
//
// The graph is deliberately not a root-cause oracle. Every edge carries its
// basis and assessment so operators can distinguish observed support,
// contradiction, and contextual relationship from causal certainty.
package evidence

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/costsim"
	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/replay"
)

const (
	AssessmentSupporting    = "supporting"
	AssessmentContradicting = "contradicting"
	AssessmentRelated       = "related"
)

// Graph is one bounded evidence view for a frozen incident.
type Graph struct {
	TenantID    string       `json:"tenant_id"`
	IncidentID  string       `json:"incident_id"`
	GeneratedAt time.Time    `json:"generated_at"`
	Disclaimer  string       `json:"disclaimer"`
	Summary     Summary      `json:"summary"`
	Nodes       []Node       `json:"nodes"`
	Edges       []Edge       `json:"edges"`
	Hypotheses  []Hypothesis `json:"hypotheses"`
}

// Summary makes the graph useful without a visual renderer.
type Summary struct {
	NodeCount          int `json:"node_count"`
	EdgeCount          int `json:"edge_count"`
	SupportingEdges    int `json:"supporting_edges"`
	ContradictingEdges int `json:"contradicting_edges"`
	RelatedEdges       int `json:"related_edges"`
}

// Node represents captured evidence or one analysis artifact.
type Node struct {
	ID         string            `json:"id"`
	Kind       string            `json:"kind"`
	Label      string            `json:"label"`
	Source     string            `json:"source,omitempty"`
	EventType  string            `json:"event_type,omitempty"`
	Timestamp  time.Time         `json:"timestamp,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// Edge is an explainable relationship between two nodes.
type Edge struct {
	From       string `json:"from"`
	To         string `json:"to"`
	Relation   string `json:"relation"`
	Assessment string `json:"assessment"`
	Reason     string `json:"reason"`
}

// Hypothesis summarizes an evidence-backed statement without claiming cause.
type Hypothesis struct {
	ID            string `json:"id"`
	Statement     string `json:"statement"`
	Status        string `json:"status"`
	Supporting    int    `json:"supporting"`
	Contradicting int    `json:"contradicting"`
	Notes         string `json:"notes"`
}

// Build creates a bounded graph from a frozen incident plus replay/cost history.
func Build(
	tenantID string,
	incidentID string,
	events []domain.Event,
	runs []replay.Run,
	simulations []costsim.Result,
) Graph {
	graph := Graph{
		TenantID:    tenantID,
		IncidentID:  incidentID,
		GeneratedAt: time.Now().UTC(),
		Disclaimer:  "Relationships are evidence associations, not automated root-cause claims.",
	}

	rootID := "incident:" + incidentID
	graph.Nodes = append(graph.Nodes, Node{
		ID: rootID, Kind: "incident", Label: incidentID,
	})

	sorted := append([]domain.Event(nil), events...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Timestamp.Equal(sorted[j].Timestamp) {
			return sorted[i].ID < sorted[j].ID
		}
		return sorted[i].Timestamp.Before(sorted[j].Timestamp)
	})

	nodeByEvent := make(map[string]string, len(sorted))
	for _, event := range sorted {
		id := "event:" + event.ID
		nodeByEvent[event.ID] = id
		attributes := make(map[string]string)
		// Correlation/trace values are used internally to construct relationships
		// but are deliberately not copied into the graph response. The edge basis
		// is useful without creating a second export of potentially sensitive IDs.
		for _, key := range []string{"deployment_id", "version", "release"} {
			if value := strings.TrimSpace(event.Tags[key]); value != "" {
				attributes[key] = value
			}
		}
		graph.Nodes = append(graph.Nodes, Node{
			ID: id, Kind: eventKind(event), Label: eventLabel(event),
			Source: event.Source, EventType: event.Type, Timestamp: event.Timestamp,
			Attributes: attributes,
		})
		graph.Edges = append(graph.Edges, Edge{
			From: rootID, To: id, Relation: "captured-in",
			Assessment: AssessmentRelated,
			Reason:     "Event is part of the frozen incident evidence window.",
		})
	}

	for _, run := range runs {
		if run.IncidentID != incidentID {
			continue
		}
		id := "replay:" + run.ID
		graph.Nodes = append(graph.Nodes, Node{
			ID: id, Kind: "replay", Label: run.ID, Timestamp: run.StartedAt,
			Attributes: map[string]string{
				"status":         run.Status,
				"active_policy":  run.ActivePolicy + "@" + run.ActiveVersion,
				"changed_events": fmt.Sprint(run.ChangedEventCount),
				"quarantined":    fmt.Sprint(run.QuarantinedCount),
			},
		})
		graph.Edges = append(graph.Edges, Edge{
			From: rootID, To: id, Relation: "evaluated-by-replay",
			Assessment: AssessmentRelated,
			Reason:     "Replay evaluated this incident under a recorded policy version.",
		})
	}

	for _, simulation := range simulations {
		if simulation.IncidentID != incidentID {
			continue
		}
		id := "cost:" + simulation.ID
		graph.Nodes = append(graph.Nodes, Node{
			ID: id, Kind: "cost-simulation", Label: simulation.ID,
			Timestamp: simulation.StartedAt,
			Attributes: map[string]string{
				"active_policy":   simulation.ActivePolicy + "@" + simulation.ActiveVersion,
				"baseline_series": fmt.Sprint(simulation.Baseline.Series),
				"active_series":   fmt.Sprint(simulation.Active.Series),
			},
		})
		graph.Edges = append(graph.Edges, Edge{
			From: rootID, To: id, Relation: "evaluated-by-cost-simulation",
			Assessment: AssessmentRelated,
			Reason:     "Cost simulation used the frozen incident as its comparison sample.",
		})
	}

	builder := relationshipBuilder{
		graph:       &graph,
		nodeByEvent: nodeByEvent,
		seen:        make(map[string]struct{}),
		maxEdges:    2000,
	}
	builder.addCorrelationEdges(sorted)
	builder.addTraceEdges(sorted)
	builder.addSourceSequenceEdges(sorted)
	builder.addSymptomEdges(sorted)
	builder.addChangeEdges(sorted)

	graph.Hypotheses = buildHypotheses(graph.Edges)
	for _, edge := range graph.Edges {
		switch edge.Assessment {
		case AssessmentSupporting:
			graph.Summary.SupportingEdges++
		case AssessmentContradicting:
			graph.Summary.ContradictingEdges++
		default:
			graph.Summary.RelatedEdges++
		}
	}
	graph.Summary.NodeCount = len(graph.Nodes)
	graph.Summary.EdgeCount = len(graph.Edges)
	return graph
}

type relationshipBuilder struct {
	graph       *Graph
	nodeByEvent map[string]string
	seen        map[string]struct{}
	maxEdges    int
}

func (builder *relationshipBuilder) add(edge Edge) {
	if len(builder.graph.Edges) >= builder.maxEdges {
		return
	}
	key := edge.From + "|" + edge.To + "|" + edge.Relation
	if _, exists := builder.seen[key]; exists {
		return
	}
	builder.seen[key] = struct{}{}
	builder.graph.Edges = append(builder.graph.Edges, edge)
}

func (builder *relationshipBuilder) addCorrelationEdges(events []domain.Event) {
	groups := make(map[string][]domain.Event)
	for _, event := range events {
		if value := strings.TrimSpace(event.CorrelationID); value != "" {
			groups[value] = append(groups[value], event)
		}
	}
	for _, group := range groups {
		builder.linkConsecutive(group, "shared-correlation",
			AssessmentSupporting,
			"Events share a captured correlation_id, supporting membership in the same application flow.")
	}
}

func (builder *relationshipBuilder) addTraceEdges(events []domain.Event) {
	groups := make(map[string][]domain.Event)
	for _, event := range events {
		if value := strings.TrimSpace(event.Tags["trace_id"]); value != "" {
			groups[value] = append(groups[value], event)
		}
	}
	for _, group := range groups {
		builder.linkConsecutive(group, "shared-trace",
			AssessmentSupporting,
			"Events share a captured trace_id, supporting membership in the same distributed trace.")
	}
}

func (builder *relationshipBuilder) addSourceSequenceEdges(events []domain.Event) {
	groups := make(map[string][]domain.Event)
	for _, event := range events {
		groups[event.Source] = append(groups[event.Source], event)
	}
	for _, group := range groups {
		sortEvents(group)
		for index := 1; index < len(group); index++ {
			previous := group[index-1]
			current := group[index]
			if current.Timestamp.Sub(previous.Timestamp) > 30*time.Second {
				continue
			}
			builder.add(Edge{
				From:       builder.nodeByEvent[previous.ID],
				To:         builder.nodeByEvent[current.ID],
				Relation:   "same-source-sequence",
				Assessment: AssessmentRelated,
				Reason:     "Events occurred on the same source within 30 seconds; this is temporal context, not causality.",
			})
		}
	}
}

func (builder *relationshipBuilder) addSymptomEdges(events []domain.Event) {
	bySource := make(map[string][]domain.Event)
	for _, event := range events {
		bySource[event.Source] = append(bySource[event.Source], event)
	}
	for _, group := range bySource {
		sortEvents(group)
		for left := 0; left < len(group); left++ {
			if highLatency(group[left]) {
				for right := left + 1; right < len(group); right++ {
					delta := group[right].Timestamp.Sub(group[left].Timestamp)
					if delta > 60*time.Second {
						break
					}
					if isError(group[right]) {
						builder.add(Edge{
							From:       builder.nodeByEvent[group[left].ID],
							To:         builder.nodeByEvent[group[right].ID],
							Relation:   "latency-precedes-error",
							Assessment: AssessmentSupporting,
							Reason:     "A high-latency signal preceded an error on the same source within 60 seconds.",
						})
						break
					}
				}
			}
			if isError(group[left]) {
				for right := left + 1; right < len(group); right++ {
					delta := group[right].Timestamp.Sub(group[left].Timestamp)
					if delta > 2*time.Minute {
						break
					}
					if normalLatency(group[right]) {
						builder.add(Edge{
							From:       builder.nodeByEvent[group[left].ID],
							To:         builder.nodeByEvent[group[right].ID],
							Relation:   "recovery-signal",
							Assessment: AssessmentContradicting,
							Reason:     "A later normal-latency signal contradicts a hypothesis of continuously sustained degradation.",
						})
						break
					}
				}
			}
		}
	}
}

func (builder *relationshipBuilder) addChangeEdges(events []domain.Event) {
	for left, change := range events {
		if !isChange(change) {
			continue
		}
		for right := left + 1; right < len(events); right++ {
			candidate := events[right]
			if candidate.Timestamp.Sub(change.Timestamp) > 5*time.Minute {
				break
			}
			if candidate.Source == change.Source && isError(candidate) {
				builder.add(Edge{
					From:       builder.nodeByEvent[change.ID],
					To:         builder.nodeByEvent[candidate.ID],
					Relation:   "change-precedes-error",
					Assessment: AssessmentSupporting,
					Reason:     "A captured deployment/change marker preceded an error on the same source within five minutes. Temporal association does not prove the change caused the error.",
				})
			}
		}
	}
}

func (builder *relationshipBuilder) linkConsecutive(
	group []domain.Event,
	relation string,
	assessment string,
	reason string,
) {
	sortEvents(group)
	for index := 1; index < len(group); index++ {
		builder.add(Edge{
			From:       builder.nodeByEvent[group[index-1].ID],
			To:         builder.nodeByEvent[group[index].ID],
			Relation:   relation,
			Assessment: assessment,
			Reason:     reason,
		})
	}
}

func buildHypotheses(edges []Edge) []Hypothesis {
	type counts struct{ support, contradict int }
	change := counts{}
	latency := counts{}
	sustained := counts{}

	for _, edge := range edges {
		switch edge.Relation {
		case "change-precedes-error":
			change.support++
		case "latency-precedes-error":
			latency.support++
			sustained.support++
		case "recovery-signal":
			sustained.contradict++
		}
	}

	result := make([]Hypothesis, 0, 3)
	if change.support > 0 {
		result = append(result, hypothesis(
			"recent-change-associated",
			"A captured change is temporally associated with errors in the incident window.",
			change.support, change.contradict,
			"Association is evidence for investigation, not proof that the change caused the incident.",
		))
	}
	if latency.support > 0 {
		result = append(result, hypothesis(
			"latency-before-errors",
			"High latency appears before errors on at least one affected source.",
			latency.support, latency.contradict,
			"This supports a symptom sequence, not a specific underlying cause.",
		))
	}
	if sustained.support > 0 || sustained.contradict > 0 {
		result = append(result, hypothesis(
			"sustained-degradation",
			"The affected source remained degraded throughout the observed failure window.",
			sustained.support, sustained.contradict,
			"Recovery signals are explicit contradictory evidence against sustained degradation.",
		))
	}
	if len(result) == 0 {
		result = append(result, Hypothesis{
			ID:        "insufficient-evidence",
			Statement: "The captured evidence does not support a stronger built-in incident hypothesis.",
			Status:    "insufficient",
			Notes:     "Absence of a built-in relationship is not evidence that no causal relationship exists.",
		})
	}
	return result
}

func hypothesis(id, statement string, support, contradict int, notes string) Hypothesis {
	status := "insufficient"
	switch {
	case support > 0 && contradict > 0:
		status = "mixed"
	case support > 0:
		status = "supported"
	case contradict > 0:
		status = "contradicted"
	}
	return Hypothesis{
		ID: id, Statement: statement, Status: status,
		Supporting: support, Contradicting: contradict, Notes: notes,
	}
}

func eventKind(event domain.Event) string {
	if isChange(event) {
		return "change"
	}
	if isError(event) {
		return "error"
	}
	if highLatency(event) {
		return "latency"
	}
	return "event"
}

func eventLabel(event domain.Event) string {
	return event.Source + " · " + event.Type
}

func highLatency(event domain.Event) bool {
	return latencyValue(event, func(value float64) bool { return value >= 1000 })
}

func normalLatency(event domain.Event) bool {
	return latencyValue(event, func(value float64) bool { return value < 500 })
}

func latencyValue(event domain.Event, test func(float64) bool) bool {
	if event.Value == nil || !strings.EqualFold(event.Unit, "ms") {
		return false
	}
	kind := strings.ToLower(event.Type)
	if !strings.Contains(kind, "latency") && !strings.Contains(kind, "duration") {
		return false
	}
	return test(*event.Value)
}

func isError(event domain.Event) bool {
	if strings.Contains(strings.ToLower(event.Type), "error") {
		return true
	}
	for _, key := range []string{"severity", "level"} {
		value := strings.ToLower(strings.TrimSpace(event.Tags[key]))
		if value == "error" || value == "critical" || value == "fatal" {
			return true
		}
	}
	return false
}

func isChange(event domain.Event) bool {
	kind := strings.ToLower(event.Type)
	if strings.Contains(kind, "deploy") ||
		strings.Contains(kind, "release") ||
		strings.Contains(kind, "change") {
		return true
	}
	for _, key := range []string{"deployment_id", "release", "version"} {
		if strings.TrimSpace(event.Tags[key]) != "" {
			return true
		}
	}
	return false
}

func sortEvents(events []domain.Event) {
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].Timestamp.Equal(events[j].Timestamp) {
			return events[i].ID < events[j].ID
		}
		return events[i].Timestamp.Before(events[j].Timestamp)
	})
}
