package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/fuhrdan/TelemetryForge/internal/storage"
)

func (server *Server) handleSchemas(writer http.ResponseWriter, request *http.Request) {
	reader, ok := server.reader.(storage.SchemaReader)
	if !ok {
		writeError(writer, http.StatusNotImplemented, "schema registry is unavailable")
		return
	}
	limit, ok := parseSchemaLimit(writer, request)
	if !ok {
		return
	}
	entries, err := reader.ListSchemas(request.Context(), limit)
	if err != nil {
		server.logger.Error("schema registry query failed", "error", err)
		writeError(writer, http.StatusServiceUnavailable, "storage backend unavailable")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"count": len(entries), "schemas": entries})
}

func (server *Server) handleSchemaHistory(writer http.ResponseWriter, request *http.Request) {
	reader, ok := server.reader.(storage.SchemaReader)
	if !ok {
		writeError(writer, http.StatusNotImplemented, "schema registry is unavailable")
		return
	}
	source := strings.TrimSpace(request.URL.Query().Get("source"))
	eventType := strings.TrimSpace(request.URL.Query().Get("type"))
	if source == "" || eventType == "" {
		writeError(writer, http.StatusBadRequest, "source and type are required")
		return
	}
	entries, err := reader.SchemaHistory(request.Context(), source, eventType)
	if err != nil {
		server.logger.Error("schema history query failed", "source", source, "type", eventType, "error", err)
		writeError(writer, http.StatusServiceUnavailable, "storage backend unavailable")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"count": len(entries), "schemas": entries})
}

func (server *Server) handleSchemaDrift(writer http.ResponseWriter, request *http.Request) {
	reader, ok := server.reader.(storage.SchemaReader)
	if !ok {
		writeError(writer, http.StatusNotImplemented, "schema registry is unavailable")
		return
	}
	limit, ok := parseSchemaLimit(writer, request)
	if !ok {
		return
	}
	drifts, err := reader.ListSchemaDrifts(request.Context(), limit)
	if err != nil {
		server.logger.Error("schema drift query failed", "error", err)
		writeError(writer, http.StatusServiceUnavailable, "storage backend unavailable")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"count": len(drifts), "drift": drifts})
}

func (server *Server) handleSchemaDiff(writer http.ResponseWriter, request *http.Request) {
	reader, ok := server.reader.(storage.SchemaReader)
	if !ok {
		writeError(writer, http.StatusNotImplemented, "schema registry is unavailable")
		return
	}
	values := request.URL.Query()
	source := strings.TrimSpace(values.Get("source"))
	eventType := strings.TrimSpace(values.Get("type"))
	fromVersion := strings.TrimSpace(values.Get("from"))
	toVersion := strings.TrimSpace(values.Get("to"))
	if source == "" || eventType == "" || fromVersion == "" || toVersion == "" {
		writeError(writer, http.StatusBadRequest, "source, type, from, and to are required")
		return
	}
	diff, err := reader.SchemaDiff(request.Context(), source, eventType, fromVersion, toVersion)
	if err != nil {
		server.logger.Error("schema diff query failed", "source", source, "type", eventType, "from", fromVersion, "to", toVersion, "error", err)
		writeError(writer, http.StatusNotFound, "schema version not found")
		return
	}
	writeJSON(writer, http.StatusOK, diff)
}

func parseSchemaLimit(writer http.ResponseWriter, request *http.Request) (int, bool) {
	limit := 100
	if raw := request.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 500 {
			writeError(writer, http.StatusBadRequest, "limit must be between 1 and 500")
			return 0, false
		}
		limit = parsed
	}
	return limit, true
}
