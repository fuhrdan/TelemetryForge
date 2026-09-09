package api

import (
	"net/http"
	"strconv"

	"github.com/fuhrdan/TelemetryForge/internal/storage"
)

func (server *Server) handleReplayRuns(writer http.ResponseWriter, request *http.Request) {
	reader, ok := server.reader.(storage.ReplayReader)
	if !ok {
		writeError(writer, http.StatusNotImplemented, "replay history is unavailable")
		return
	}
	limit, ok := parseHistoryLimit(writer, request)
	if !ok {
		return
	}

	runs, err := reader.ListReplayRuns(request.Context(), limit)
	if err != nil {
		server.logger.Error("replay history query failed", "error", err)
		writeError(writer, http.StatusServiceUnavailable, "storage backend unavailable")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"count": len(runs), "runs": runs})
}

func (server *Server) handleCostSimulations(writer http.ResponseWriter, request *http.Request) {
	reader, ok := server.reader.(storage.ReplayReader)
	if !ok {
		writeError(writer, http.StatusNotImplemented, "cost simulation history is unavailable")
		return
	}
	limit, ok := parseHistoryLimit(writer, request)
	if !ok {
		return
	}

	results, err := reader.ListCostSimulations(request.Context(), limit)
	if err != nil {
		server.logger.Error("cost simulation query failed", "error", err)
		writeError(writer, http.StatusServiceUnavailable, "storage backend unavailable")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"count": len(results), "simulations": results})
}

func parseHistoryLimit(writer http.ResponseWriter, request *http.Request) (int, bool) {
	limit := 25
	if raw := request.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 200 {
			writeError(writer, http.StatusBadRequest, "limit must be between 1 and 200")
			return 0, false
		}
		limit = parsed
	}
	return limit, true
}
