package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/changeintel"
	"github.com/fuhrdan/TelemetryForge/internal/costsim"
	"github.com/fuhrdan/TelemetryForge/internal/evidence"
	"github.com/fuhrdan/TelemetryForge/internal/intelligence"
	"github.com/fuhrdan/TelemetryForge/internal/replay"
	"github.com/fuhrdan/TelemetryForge/internal/security"
	"github.com/fuhrdan/TelemetryForge/internal/storage"
)

func (server *Server) handleInvestigation(writer http.ResponseWriter, request *http.Request) {
	incidentID := request.PathValue("id")
	if incidentID == "" {
		writeError(writer, http.StatusBadRequest, "incident id is required")
		return
	}

	graph, runs, simulations, err := server.intelligenceInputs(request.Context(), incidentID)
	if err != nil {
		server.logger.Error("intelligence investigation failed", "incident_id", incidentID, "error", err)
		writeError(writer, http.StatusServiceUnavailable, err.Error())
		return
	}
	if len(graph.Nodes) == 0 {
		writeError(writer, http.StatusNotFound, "incident not found or contains no evidence")
		return
	}

	result := intelligence.Investigate(graph, runs, simulations)
	if sink, ok := server.reader.(storage.IntelligenceStore); ok {
		if err := sink.SaveInvestigation(request.Context(), result); err != nil {
			server.logger.Warn("intelligence snapshot persistence failed", "incident_id", incidentID, "error", err)
		}
	}
	writeJSON(writer, http.StatusOK, result)
}

func (server *Server) handleInvestigationHistory(writer http.ResponseWriter, request *http.Request) {
	reader, ok := server.reader.(storage.IntelligenceStore)
	if !ok {
		writeError(writer, http.StatusNotImplemented, "intelligence history is unavailable")
		return
	}
	limit, ok := parseHistoryLimit(writer, request)
	if !ok {
		return
	}
	items, err := reader.ListInvestigations(request.Context(), limit)
	if err != nil {
		server.logger.Error("intelligence history failed", "error", err)
		writeError(writer, http.StatusServiceUnavailable, "storage backend unavailable")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"count": len(items), "investigations": items})
}

func (server *Server) handleIncidentComparison(writer http.ResponseWriter, request *http.Request) {
	leftID := request.PathValue("id")
	rightID := request.URL.Query().Get("other")
	if leftID == "" || rightID == "" {
		writeError(writer, http.StatusBadRequest, "incident id and ?other= incident id are required")
		return
	}
	if leftID == rightID {
		writeError(writer, http.StatusBadRequest, "comparison requires two different incidents")
		return
	}

	left, _, _, err := server.intelligenceInputs(request.Context(), leftID)
	if err != nil {
		writeError(writer, http.StatusServiceUnavailable, err.Error())
		return
	}
	right, _, _, err := server.intelligenceInputs(request.Context(), rightID)
	if err != nil {
		writeError(writer, http.StatusServiceUnavailable, err.Error())
		return
	}
	if len(left.Nodes) == 0 || len(right.Nodes) == 0 {
		writeError(writer, http.StatusNotFound, "one or both incidents contain no evidence")
		return
	}
	writeJSON(writer, http.StatusOK, intelligence.Compare(left, right))
}

func (server *Server) intelligenceInputs(ctx context.Context, incidentID string) (evidence.Graph, []replay.Run, []costsim.Result, error) {
	dashboard, ok := server.reader.(storage.DashboardReader)
	if !ok {
		return evidence.Graph{}, nil, nil, fmt.Errorf("incident storage is unavailable")
	}
	events, err := dashboard.IncidentEvents(ctx, incidentID, 1000)
	if err != nil {
		return evidence.Graph{}, nil, nil, fmt.Errorf("load incident events: %w", err)
	}
	if len(events) == 0 {
		return evidence.Graph{}, nil, nil, nil
	}

	var runs []replay.Run
	var simulations []costsim.Result
	if history, ok := server.reader.(storage.ReplayReader); ok {
		runs, _ = history.ListReplayRuns(ctx, 200)
		simulations, _ = history.ListCostSimulations(ctx, 200)
	}

	var changes []changeintel.Marker
	if reader, ok := server.reader.(storage.ChangeIntelligenceReader); ok {
		from, to := events[0].Timestamp, events[0].Timestamp
		for _, event := range events[1:] {
			if event.Timestamp.Before(from) {
				from = event.Timestamp
			}
			if event.Timestamp.After(to) {
				to = event.Timestamp
			}
		}
		changes, _ = reader.ChangesBetween(ctx, from.Add(-15*time.Minute), to.Add(5*time.Minute), 100)
	}

	graph := evidence.BuildWithChanges(security.TenantID(ctx), incidentID, events, runs, simulations, changes)
	if sink, ok := server.reader.(storage.EvidenceStore); ok {
		_ = sink.SaveEvidenceGraph(ctx, graph)
	}
	return graph, runs, simulations, nil
}
