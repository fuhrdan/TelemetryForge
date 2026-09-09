package api

import (
	"net/http"

	"github.com/fuhrdan/TelemetryForge/internal/storage"
)

func (server *Server) handleRoutingDeliveries(writer http.ResponseWriter, request *http.Request) {
	reader, ok := server.reader.(storage.RoutingReader)
	if !ok {
		writeError(writer, http.StatusNotImplemented, "routing storage is unavailable")
		return
	}
	limit, ok := parsePolicyLimit(writer, request)
	if !ok {
		return
	}
	status := request.URL.Query().Get("status")
	switch status {
	case "", "pending", "sending", "retry", "delivered", "dead_letter":
	default:
		writeError(writer, http.StatusBadRequest, "status must be pending, sending, retry, delivered, or dead_letter")
		return
	}
	deliveries, err := reader.ListRoutingDeliveries(request.Context(), status, limit)
	if err != nil {
		server.logger.Error("routing delivery query failed", "error", err)
		writeError(writer, http.StatusServiceUnavailable, "storage backend unavailable")
		return
	}
	if server.redactor != nil {
		for index := range deliveries {
			deliveries[index].Event = server.redactor.Event(deliveries[index].Event)
		}
	}
	writeJSON(writer, http.StatusOK, map[string]any{"count": len(deliveries), "status": status, "deliveries": deliveries})
}

func (server *Server) handleRoutingShadowDiffs(writer http.ResponseWriter, request *http.Request) {
	reader, ok := server.reader.(storage.RoutingReader)
	if !ok {
		writeError(writer, http.StatusNotImplemented, "routing storage is unavailable")
		return
	}
	limit, ok := parsePolicyLimit(writer, request)
	if !ok {
		return
	}
	diffs, err := reader.ListRoutingShadowDiffs(request.Context(), limit)
	if err != nil {
		server.logger.Error("routing shadow diff query failed", "error", err)
		writeError(writer, http.StatusServiceUnavailable, "storage backend unavailable")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"count": len(diffs), "diffs": diffs})
}

func (server *Server) handleRoutingDeadLetters(writer http.ResponseWriter, request *http.Request) {
	reader, ok := server.reader.(storage.RoutingReader)
	if !ok {
		writeError(writer, http.StatusNotImplemented, "routing storage is unavailable")
		return
	}
	limit, ok := parsePolicyLimit(writer, request)
	if !ok {
		return
	}
	destination := request.URL.Query().Get("destination")
	letters, err := reader.ListRoutingDeadLetters(request.Context(), destination, limit)
	if err != nil {
		server.logger.Error("routing dead-letter query failed", "error", err)
		writeError(writer, http.StatusServiceUnavailable, "storage backend unavailable")
		return
	}
	if server.redactor != nil {
		for index := range letters {
			letters[index].Event = server.redactor.Event(letters[index].Event)
		}
	}
	writeJSON(writer, http.StatusOK, map[string]any{"count": len(letters), "destination": destination, "dead_letters": letters})
}

func (server *Server) handleRoutingHealth(writer http.ResponseWriter, request *http.Request) {
	reader, ok := server.reader.(storage.RoutingReader)
	if !ok {
		writeError(writer, http.StatusNotImplemented, "routing storage is unavailable")
		return
	}
	limit, ok := parsePolicyLimit(writer, request)
	if !ok {
		return
	}
	destinations, err := reader.ListRoutingDestinationHealth(request.Context(), limit)
	if err != nil {
		server.logger.Error("routing health query failed", "error", err)
		writeError(writer, http.StatusServiceUnavailable, "storage backend unavailable")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"count": len(destinations), "destinations": destinations})
}
