package api

import (
	"net/http"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/changeintel"
	"github.com/fuhrdan/TelemetryForge/internal/costsim"
	"github.com/fuhrdan/TelemetryForge/internal/evidence"
	"github.com/fuhrdan/TelemetryForge/internal/replay"
	"github.com/fuhrdan/TelemetryForge/internal/security"
	"github.com/fuhrdan/TelemetryForge/internal/storage"
)

func (server *Server) handleEvidenceGraph(writer http.ResponseWriter, request *http.Request) {
	dashboard, ok := server.reader.(storage.DashboardReader)
	if !ok {
		writeError(writer, http.StatusNotImplemented, "incident storage is unavailable")
		return
	}

	incidentID := request.PathValue("id")
	if incidentID == "" {
		writeError(writer, http.StatusBadRequest, "incident id is required")
		return
	}

	events, err := dashboard.IncidentEvents(request.Context(), incidentID, 1000)
	if err != nil {
		server.logger.Error("evidence graph incident query failed", "incident_id", incidentID, "error", err)
		writeError(writer, http.StatusServiceUnavailable, "storage backend unavailable")
		return
	}
	if len(events) == 0 {
		writeError(writer, http.StatusNotFound, "incident not found or contains no events")
		return
	}

	var runs []replay.Run
	var simulations []costsim.Result
	if history, ok := server.reader.(storage.ReplayReader); ok {
		runs, _ = history.ListReplayRuns(request.Context(), 200)
		simulations, _ = history.ListCostSimulations(request.Context(), 200)
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
		changes, _ = reader.ChangesBetween(request.Context(), from.Add(-15*time.Minute), to.Add(5*time.Minute), 100)
	}

	graph := evidence.BuildWithChanges(
		security.TenantID(request.Context()), incidentID, events, runs, simulations, changes,
	)

	if sink, ok := server.reader.(storage.EvidenceStore); ok {
		if err := sink.SaveEvidenceGraph(request.Context(), graph); err != nil {
			server.logger.Warn("evidence graph snapshot persistence failed",
				"incident_id", incidentID, "error", err)
		}
	}

	writeJSON(writer, http.StatusOK, graph)
}
