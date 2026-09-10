// Package api implements the HTTP ingestion surface for TelemetryForge.
package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/security"
	"github.com/fuhrdan/TelemetryForge/internal/storage"
	"github.com/fuhrdan/TelemetryForge/internal/stream"
)

const (
	maxRequestBodyBytes int64 = 1 << 20 // 1 MiB
	readinessTimeout          = 2 * time.Second
)

// Topics maps API event classes to their Kafka topics.
type Topics struct {
	Raw    string
	Metric string
}

// Observer receives semantic gateway events without coupling the API package to
// a particular metrics implementation.
type Observer interface {
	Accepted(kind, topic string)
	PublishFailure(topic string)
}

// Server owns the HTTP routes for the TelemetryForge gateway.
type Server struct {
	logger    *slog.Logger
	mux       *http.ServeMux
	publisher stream.Publisher
	topics    Topics
	reader    storage.Reader
	observer  Observer
	redactor  *security.Redactor
}

// NewServer creates a configured HTTP server handler.
func NewServer(logger *slog.Logger, publisher stream.Publisher, topics Topics, readers ...storage.Reader) *Server {
	var reader storage.Reader
	if len(readers) > 0 {
		reader = readers[0]
	}
	return NewServerWithObserver(logger, publisher, topics, reader, nil)
}

// NewServerWithObserver creates a gateway with semantic observability hooks.
func NewServerWithObserver(logger *slog.Logger, publisher stream.Publisher, topics Topics, reader storage.Reader, observer Observer) *Server {
	server := &Server{
		logger: logger, mux: http.NewServeMux(), publisher: publisher,
		topics: topics, reader: reader, observer: observer,
	}
	server.routes()
	return server
}

// SetRedactor enables presentation-time API redaction. It does not modify
// stored telemetry or Flight Recorder evidence.
func (server *Server) SetRedactor(redactor *security.Redactor) {
	server.redactor = redactor
}

func (server *Server) redactEvents(events []domain.Event) []domain.Event {
	if server.redactor == nil {
		return events
	}
	return server.redactor.Events(events)
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
	server.mux.HandleFunc("POST /api/v1/changes", server.handleChange)
	if server.reader != nil {
		server.mux.HandleFunc("GET /api/v1/events", server.handleQueryEvents)
		server.mux.HandleFunc("GET /api/v1/metrics", server.handleQueryMetrics)
		server.mux.HandleFunc("GET /api/v1/dashboard/summary", server.handleSummary)
		server.mux.HandleFunc("GET /api/v1/incidents", server.handleIncidents)
		server.mux.HandleFunc("GET /api/v1/incidents/{id}/events", server.handleIncidentEvents)
		server.mux.HandleFunc("GET /api/v1/incidents/{id}/evidence-graph", server.handleEvidenceGraph)
		server.mux.HandleFunc("GET /api/v1/incidents/{id}/changes", server.handleIncidentChanges)
		server.mux.HandleFunc("GET /api/v1/live", server.handleLive)
		if _, ok := server.reader.(storage.PolicyReader); ok {
			server.mux.HandleFunc("GET /api/v1/cardinality/findings", server.handleCardinalityFindings)
			server.mux.HandleFunc("GET /api/v1/cardinality/state", server.handleCardinalityStates)
			server.mux.HandleFunc("GET /api/v1/cardinality/budgets", server.handleCardinalityBudgets)
			server.mux.HandleFunc("GET /api/v1/policy/shadow-diffs", server.handlePolicyDiffs)
		}
		if _, ok := server.reader.(storage.ReplayReader); ok {
			server.mux.HandleFunc("GET /api/v1/replays", server.handleReplayRuns)
			server.mux.HandleFunc("GET /api/v1/cost-simulations", server.handleCostSimulations)
		}
		if _, ok := server.reader.(storage.IncidentArchiveReader); ok {
			server.mux.HandleFunc("GET /api/v1/archive-imports", server.handleArchiveImports)
		}
		if _, ok := server.reader.(storage.ChangeIntelligenceReader); ok {
			server.mux.HandleFunc("GET /api/v1/changes", server.handleChanges)
			server.mux.HandleFunc("GET /api/v1/changes/{id}/analysis", server.handleChangeAnalysis)
			server.mux.HandleFunc("POST /api/v1/changes/{id}/analyze", server.handleAnalyzeChange)
		}
		if _, ok := server.reader.(storage.SchemaReader); ok {
			server.mux.HandleFunc("GET /api/v1/schemas", server.handleSchemas)
			server.mux.HandleFunc("GET /api/v1/schema-history", server.handleSchemaHistory)
			server.mux.HandleFunc("GET /api/v1/schema-drift", server.handleSchemaDrift)
			server.mux.HandleFunc("GET /api/v1/schema-diff", server.handleSchemaDiff)
		}
		if _, ok := server.reader.(storage.ShapingReader); ok {
			server.mux.HandleFunc("GET /api/v1/shaping/stats", server.handleShapingStats)
			server.mux.HandleFunc("GET /api/v1/shaping/shadow-diffs", server.handleShapingShadowDiffs)
		}
		if _, ok := server.reader.(storage.RoutingReader); ok {
			server.mux.HandleFunc("GET /api/v1/routing/deliveries", server.handleRoutingDeliveries)
			server.mux.HandleFunc("GET /api/v1/routing/shadow-diffs", server.handleRoutingShadowDiffs)
			server.mux.HandleFunc("GET /api/v1/routing/dead-letters", server.handleRoutingDeadLetters)
			server.mux.HandleFunc("GET /api/v1/routing/destinations", server.handleRoutingHealth)
		}
	}
}

func (server *Server) handleHealth(writer http.ResponseWriter, _ *http.Request) {
	writeJSON(writer, http.StatusOK, map[string]string{"status": "ok"})
}

func (server *Server) handleReady(writer http.ResponseWriter, request *http.Request) {
	ctx, cancel := context.WithTimeout(request.Context(), readinessTimeout)
	defer cancel()

	if err := server.publisher.Ready(ctx); err != nil {
		server.logger.Warn("gateway not ready", "dependency", "kafka", "error", err)
		writeJSON(writer, http.StatusServiceUnavailable, map[string]string{
			"status":     "not_ready",
			"dependency": "kafka",
		})
		return
	}

	// The gateway serves stored queries, incidents, dashboard summaries, and SSE
	// when a database-backed reader is configured. Kubernetes readiness must
	// therefore include that dependency rather than checking Kafka alone.
	if checker, ok := server.reader.(interface {
		Ready(context.Context) error
	}); ok {
		if err := checker.Ready(ctx); err != nil {
			server.logger.Warn("gateway not ready", "dependency", "database", "error", err)
			writeJSON(writer, http.StatusServiceUnavailable, map[string]string{
				"status":     "not_ready",
				"dependency": "database",
			})
			return
		}
	}

	writeJSON(writer, http.StatusOK, map[string]string{"status": "ready"})
}

func (server *Server) handleEvent(writer http.ResponseWriter, request *http.Request) {
	event, err := decodeEvent(writer, request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}

	if err := authorizeEventTenant(request, &event); err != nil {
		writeError(writer, http.StatusForbidden, err.Error())
		return
	}

	if err := ensureEventID(&event); err != nil {
		server.logger.Error("event id generation failed", "error", err)
		writeError(writer, http.StatusInternalServerError, "unable to accept event")
		return
	}

	if err := server.publisher.Publish(request.Context(), server.topics.Raw, event); err != nil {
		if server.observer != nil {
			server.observer.PublishFailure(server.topics.Raw)
		}
		server.logger.Error("event publish failed", "event_id", event.ID, "topic", server.topics.Raw, "error", err)
		writeError(writer, http.StatusServiceUnavailable, "streaming backend unavailable")
		return
	}

	if server.observer != nil {
		server.observer.Accepted("event", server.topics.Raw)
	}
	server.logger.Info(
		"telemetry event accepted",
		"tenant_id", event.TenantID,
		"event_id", event.ID,
		"source", event.Source,
		"type", event.Type,
		"schema_version", event.SchemaVersion,
		"correlation_id", event.CorrelationID,
		"topic", server.topics.Raw,
	)

	writeAccepted(writer, event.ID)
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

	if err := authorizeEventTenant(request, &event); err != nil {
		writeError(writer, http.StatusForbidden, err.Error())
		return
	}

	if err := ensureEventID(&event); err != nil {
		server.logger.Error("event id generation failed", "error", err)
		writeError(writer, http.StatusInternalServerError, "unable to accept metric")
		return
	}

	if err := server.publisher.Publish(request.Context(), server.topics.Metric, event); err != nil {
		if server.observer != nil {
			server.observer.PublishFailure(server.topics.Metric)
		}
		server.logger.Error("metric publish failed", "event_id", event.ID, "topic", server.topics.Metric, "error", err)
		writeError(writer, http.StatusServiceUnavailable, "streaming backend unavailable")
		return
	}

	if server.observer != nil {
		server.observer.Accepted("metric", server.topics.Metric)
	}
	server.logger.Info(
		"metric accepted",
		"tenant_id", event.TenantID,
		"event_id", event.ID,
		"source", event.Source,
		"type", event.Type,
		"value", *event.Value,
		"unit", event.Unit,
		"topic", server.topics.Metric,
	)

	writeAccepted(writer, event.ID)
}

func authorizeEventTenant(request *http.Request, event *domain.Event) error {
	if event.TenantID != "" {
		return errors.New("tenant_id must be omitted; it is assigned by authentication")
	}
	event.TenantID = security.TenantID(request.Context())
	return nil
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

func ensureEventID(event *domain.Event) error {
	if event.ID != "" {
		return nil
	}

	id, err := newEventID()
	if err != nil {
		return err
	}
	event.ID = id
	return nil
}

func newEventID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

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

func writeAccepted(writer http.ResponseWriter, id string) {
	writeJSON(writer, http.StatusAccepted, map[string]string{"id": id, "status": "accepted"})
}

func writeError(writer http.ResponseWriter, status int, message string) {
	writeJSON(writer, status, map[string]string{"error": message})
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}
