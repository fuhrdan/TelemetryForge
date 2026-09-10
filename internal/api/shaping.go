package api

import (
	"net/http"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/storage"
)

func (server *Server) handleShapingStats(writer http.ResponseWriter, request *http.Request) {
	reader, ok := server.reader.(storage.ShapingReader)
	if !ok {
		writeError(writer, http.StatusNotImplemented, "shaping storage is unavailable")
		return
	}
	limit, ok := parsePolicyLimit(writer, request)
	if !ok {
		return
	}
	window := time.Hour
	if raw := request.URL.Query().Get("window"); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil || parsed <= 0 || parsed > 30*24*time.Hour {
			writeError(writer, http.StatusBadRequest, "window must be a positive duration no greater than 720h")
			return
		}
		window = parsed
	}
	from := time.Now().UTC().Add(-window)
	summary, err := reader.ShapingSummary(request.Context(), from)
	if err != nil {
		server.logger.Error("shaping summary query failed", "error", err)
		writeError(writer, http.StatusServiceUnavailable, "storage backend unavailable")
		return
	}
	stats, err := reader.ListShapingStats(request.Context(), from, limit)
	if err != nil {
		server.logger.Error("shaping stats query failed", "error", err)
		writeError(writer, http.StatusServiceUnavailable, "storage backend unavailable")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"count": len(stats), "window_seconds": int64(window.Seconds()), "summary": summary, "stats": stats})
}

func (server *Server) handleShapingShadowDiffs(writer http.ResponseWriter, request *http.Request) {
	reader, ok := server.reader.(storage.ShapingReader)
	if !ok {
		writeError(writer, http.StatusNotImplemented, "shaping storage is unavailable")
		return
	}
	limit, ok := parsePolicyLimit(writer, request)
	if !ok {
		return
	}
	diffs, err := reader.ListShapingShadowDiffs(request.Context(), limit)
	if err != nil {
		server.logger.Error("shaping shadow diff query failed", "error", err)
		writeError(writer, http.StatusServiceUnavailable, "storage backend unavailable")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"count": len(diffs), "diffs": diffs})
}
