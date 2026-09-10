package api

import (
	"net/http"

	"github.com/fuhrdan/TelemetryForge/internal/connectors"
	"github.com/fuhrdan/TelemetryForge/internal/storage"
)

func (server *Server) handleConnectorCatalog(writer http.ResponseWriter, _ *http.Request) {
	catalog := connectors.Catalog()
	writeJSON(writer, http.StatusOK, map[string]any{"count": len(catalog), "connectors": catalog})
}

func (server *Server) handleConnectorRuntime(writer http.ResponseWriter, request *http.Request) {
	reader, ok := server.reader.(storage.ConnectorReader)
	if !ok {
		writeError(writer, http.StatusNotImplemented, "connector runtime state is unavailable")
		return
	}
	limit, ok := parseHistoryLimit(writer, request)
	if !ok {
		return
	}
	states, err := reader.ListConnectorRuntimeStates(request.Context(), limit)
	if err != nil {
		server.logger.Error("connector runtime query failed", "error", err)
		writeError(writer, http.StatusServiceUnavailable, "storage backend unavailable")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"count": len(states), "connectors": states})
}
