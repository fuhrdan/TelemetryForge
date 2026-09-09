// Package api implements the HTTP ingestion surface for TelemetryForge.
package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
)

const maxRequestBodyBytes int64 = 1 << 20 // 1 MiB

// Server owns the HTTP routes for the TelemetryForge gateway.
type Server struct {
	logger *slog.Logger
	mux    *http.ServeMux
}

// NewServer creates a configured HTTP server handler.
func NewServer(logger *slog.Logger) *Server {
	server := &Server{
		logger: logger,
		mux:    http.NewServeMux(),
	}

	server.routes()
	return server
}

// ServeHTTP implements http.Handler.
func (server *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	server.mux.ServeHTTP(writer, request)
}

func (server *Server) routes() {
	server.mux.HandleFunc("GET /health", server.handleHealth)
	server.mux.HandleFunc("GET /ready", server.handleReady)
	server.mux.HandleFunc("POST /api/v1/events", server.handleEvent)
	server.mux.HandleFunc("POST /api/v1/metrics", server.handleMetric)
}

func (server *Server) handleHealth(writer http.ResponseWriter, request *http.Request) {
	writeJSON(writer, http.StatusOK, map[string]string{"status": "ok"})
}

func (server *Server) handleReady(writer http.ResponseWriter, request *http.Request) {
	// v0.1.0 has no downstream dependencies yet. Later releases will extend
	// readiness checks to Kafka, storage, and other required infrastructure.
	writeJSON(writer, http.StatusOK, map[string]string{"status": "ready"})
}

func (server *Server) handleEvent(writer http.ResponseWriter, request *http.Request) {
	event, err := decodeEvent(writer, request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}

	if event.ID == "" {
		event.ID, err = newEventID()
		if err != nil {
			server.logger.Error("event id generation failed", "error", err)
			writeError(writer, http.StatusInternalServerError, "unable to accept event")
			return
		}
	}

	server.logger.Info(
		"telemetry event accepted",
		"event_id", event.ID,
		"source", event.Source,
		"type", event.Type,
		"schema_version", event.SchemaVersion,
		"correlation_id", event.CorrelationID,
	)

	writeJSON(writer, http.StatusAccepted, map[string]string{
		"id":     event.ID,
		"status": "accepted",
	})
}

func (server *Server) handleMetric(writer http.ResponseWriter, request *http.Request) {
	event, err := decodeEvent(writer, request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}

	if event.Value == nil {
		writeError(writer, http.StatusBadRequest, "value is required for metric events")
		return
	}

	if event.ID == "" {
		event.ID, err = newEventID()
		if err != nil {
			server.logger.Error("event id generation failed", "error", err)
			writeError(writer, http.StatusInternalServerError, "unable to accept metric")
			return
		}
	}

	server.logger.Info(
		"metric accepted",
		"event_id", event.ID,
		"source", event.Source,
		"type", event.Type,
		"value", *event.Value,
		"unit", event.Unit,
	)

	writeJSON(writer, http.StatusAccepted, map[string]string{
		"id":     event.ID,
		"status": "accepted",
	})
}

func decodeEvent(writer http.ResponseWriter, request *http.Request) (domain.Event, error) {
	request.Body = http.MaxBytesReader(writer, request.Body, maxRequestBodyBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()

	var event domain.Event
	if err := decoder.Decode(&event); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			return domain.Event{}, errors.New("request body exceeds 1 MiB")
		}

		return domain.Event{}, errors.New("invalid JSON payload")
	}

	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return domain.Event{}, errors.New("request body must contain exactly one JSON object")
	}

	if err := event.Validate(); err != nil {
		return domain.Event{}, err
	}

	return event, nil
}

func newEventID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	// Set UUIDv4-compatible version and variant bits without adding a runtime
	// dependency solely for identifier generation.
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80

	encoded := make([]byte, 36)
	hex.Encode(encoded[0:8], bytes[0:4])
	encoded[8] = '-'
	hex.Encode(encoded[9:13], bytes[4:6])
	encoded[13] = '-'
	hex.Encode(encoded[14:18], bytes[6:8])
	encoded[18] = '-'
	hex.Encode(encoded[19:23], bytes[8:10])
	encoded[23] = '-'
	hex.Encode(encoded[24:36], bytes[10:16])
	return string(encoded), nil
}

func writeError(writer http.ResponseWriter, status int, message string) {
	writeJSON(writer, status, map[string]string{"error": message})
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}
