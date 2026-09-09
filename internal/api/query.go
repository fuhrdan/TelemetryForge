package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/storage"
)

const maxQueryLimit = 1000

func (server *Server) handleQueryEvents(writer http.ResponseWriter, request *http.Request) {
	server.queryTelemetry(writer, request, false)
}

func (server *Server) handleQueryMetrics(writer http.ResponseWriter, request *http.Request) {
	server.queryTelemetry(writer, request, true)
}

func (server *Server) queryTelemetry(writer http.ResponseWriter, request *http.Request, metricsOnly bool) {
	query, err := parseStorageQuery(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}

	events, err := server.reader.QueryEvents(request.Context(), query)
	if err != nil {
		server.logger.Error("telemetry query failed", "error", err)
		writeError(writer, http.StatusServiceUnavailable, "storage backend unavailable")
		return
	}

	if metricsOnly {
		filtered := events[:0]
		for _, event := range events {
			if event.Value != nil {
				filtered = append(filtered, event)
			}
		}
		events = filtered
	}

	writeJSON(writer, http.StatusOK, map[string]any{
		"count":  len(events),
		"events": events,
	})
}

// parseStorageQuery converts URL parameters into a typed storage query.
//
// Time values use RFC3339 to keep the public API unambiguous across time zones.
func parseStorageQuery(request *http.Request) (storage.Query, error) {
	values := request.URL.Query()
	query := storage.Query{
		Source: values.Get("source"),
		Type:   values.Get("type"),
		Limit:  100,
	}

	var err error
	if raw := values.Get("from"); raw != "" {
		query.From, err = time.Parse(time.RFC3339, raw)
		if err != nil {
			return storage.Query{}, errors.New("from must be an RFC3339 timestamp")
		}
	}
	if raw := values.Get("to"); raw != "" {
		query.To, err = time.Parse(time.RFC3339, raw)
		if err != nil {
			return storage.Query{}, errors.New("to must be an RFC3339 timestamp")
		}
	}
	if raw := values.Get("limit"); raw != "" {
		query.Limit, err = strconv.Atoi(raw)
		if err != nil || query.Limit < 1 || query.Limit > maxQueryLimit {
			return storage.Query{}, errors.New("limit must be between 1 and 1000")
		}
	}
	if !query.From.IsZero() && !query.To.IsZero() && query.From.After(query.To) {
		return storage.Query{}, errors.New("from must not be later than to")
	}
	return query, nil
}
