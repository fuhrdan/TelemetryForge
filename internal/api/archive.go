package api

import (
	"net/http"

	"github.com/fuhrdan/TelemetryForge/internal/storage"
)

func (server *Server) handleArchiveImports(writer http.ResponseWriter, request *http.Request) {
	reader, ok := server.reader.(storage.IncidentArchiveReader)
	if !ok {
		writeError(writer, http.StatusNotImplemented, "incident archive history is unavailable")
		return
	}

	limit, ok := parseHistoryLimit(writer, request)
	if !ok {
		return
	}

	imports, err := reader.ListIncidentArchiveImports(request.Context(), limit)
	if err != nil {
		server.logger.Error("incident archive import history failed", "error", err)
		writeError(writer, http.StatusServiceUnavailable, "storage backend unavailable")
		return
	}

	writeJSON(writer, http.StatusOK, map[string]any{
		"count":   len(imports),
		"imports": imports,
	})
}
