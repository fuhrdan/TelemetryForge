package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/changeintel"
	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/security"
	"github.com/fuhrdan/TelemetryForge/internal/storage"
)

type changeRequest struct {
	ChangeID        string            `json:"change_id,omitempty"`
	Source          string            `json:"source"`
	Kind            string            `json:"kind"`
	Status          string            `json:"status,omitempty"`
	Timestamp       time.Time         `json:"timestamp,omitempty"`
	Environment     string            `json:"environment,omitempty"`
	Version         string            `json:"version,omitempty"`
	PreviousVersion string            `json:"previous_version,omitempty"`
	GitSHA          string            `json:"git_sha,omitempty"`
	BuildID         string            `json:"build_id,omitempty"`
	Actor           string            `json:"actor,omitempty"`
	RollbackOf      string            `json:"rollback_of,omitempty"`
	Summary         string            `json:"summary,omitempty"`
	CorrelationID   string            `json:"correlation_id,omitempty"`
	Tags            map[string]string `json:"tags,omitempty"`
}

func (server *Server) handleChange(writer http.ResponseWriter, request *http.Request) {
	request.Body = http.MaxBytesReader(writer, request.Body, maxRequestBodyBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var input changeRequest
	if err := decoder.Decode(&input); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(writer, http.StatusRequestEntityTooLarge, "request body exceeds 1 MiB")
			return
		}
		writeError(writer, http.StatusBadRequest, "invalid change JSON payload")
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeError(writer, http.StatusBadRequest, "request body must contain exactly one JSON object")
		return
	}

	input.Source = strings.TrimSpace(input.Source)
	input.Kind = strings.ToLower(strings.TrimSpace(input.Kind))
	input.Status = strings.ToLower(strings.TrimSpace(input.Status))
	if input.Source == "" {
		writeError(writer, http.StatusBadRequest, "source is required")
		return
	}
	if input.Status == "" {
		input.Status = "completed"
	}
	if !validChangeStatus(input.Status) {
		writeError(writer, http.StatusBadRequest, "status must be started, completed, failed, rolled_back, or changed")
		return
	}
	eventType, ok := changeEventType(input.Kind, input.Status)
	if !ok {
		writeError(writer, http.StatusBadRequest, "kind must be deployment, rollback, release, feature_flag, configuration, or infrastructure")
		return
	}
	if input.Timestamp.IsZero() {
		input.Timestamp = time.Now().UTC()
	}
	if input.Timestamp.After(time.Now().UTC().Add(5 * time.Minute)) {
		writeError(writer, http.StatusBadRequest, "timestamp cannot be more than five minutes in the future")
		return
	}

	eventID, err := newEventID()
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "unable to generate event id")
		return
	}
	changeID := strings.TrimSpace(input.ChangeID)
	if changeID == "" {
		changeID = "CHG-" + eventID
	}

	tags := make(map[string]string, len(input.Tags)+10)
	for key, value := range input.Tags {
		if strings.TrimSpace(key) != "" {
			tags[key] = value
		}
	}
	reserved := map[string]string{
		"telemetryforge.change_id":        changeID,
		"telemetryforge.change_kind":      input.Kind,
		"telemetryforge.change_status":    input.Status,
		"telemetryforge.environment":      strings.TrimSpace(input.Environment),
		"telemetryforge.version":          strings.TrimSpace(input.Version),
		"telemetryforge.previous_version": strings.TrimSpace(input.PreviousVersion),
		"telemetryforge.git_sha":          strings.TrimSpace(input.GitSHA),
		"telemetryforge.build_id":         strings.TrimSpace(input.BuildID),
		"telemetryforge.change_actor":     strings.TrimSpace(input.Actor),
		"telemetryforge.rollback_of":      strings.TrimSpace(input.RollbackOf),
	}
	for key, value := range reserved {
		if value != "" {
			tags[key] = value
		}
	}
	payload, _ := json.Marshal(map[string]string{"summary": strings.TrimSpace(input.Summary)})
	event := domain.Event{
		ID:            eventID,
		TenantID:      security.TenantID(request.Context()),
		Source:        input.Source,
		Type:          eventType,
		Timestamp:     input.Timestamp.UTC(),
		Tags:          tags,
		Payload:       payload,
		SchemaVersion: "1.0",
		CorrelationID: strings.TrimSpace(input.CorrelationID),
	}
	if err := event.Validate(); err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	if err := server.publisher.Publish(request.Context(), server.topics.Raw, event); err != nil {
		if server.observer != nil {
			server.observer.PublishFailure(server.topics.Raw)
		}
		server.logger.Error("change publish failed", "change_id", changeID, "event_id", eventID, "error", err)
		writeError(writer, http.StatusServiceUnavailable, "streaming backend unavailable")
		return
	}
	if server.observer != nil {
		server.observer.Accepted("change", server.topics.Raw)
	}
	writeJSON(writer, http.StatusAccepted, map[string]string{"change_id": changeID, "event_id": eventID, "status": "accepted"})
}

func (server *Server) handleChanges(writer http.ResponseWriter, request *http.Request) {
	reader, ok := server.reader.(storage.ChangeIntelligenceReader)
	if !ok {
		writeError(writer, http.StatusNotImplemented, "change intelligence is unavailable")
		return
	}
	limit := 50
	if raw := request.URL.Query().Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 200 {
			writeError(writer, http.StatusBadRequest, "limit must be between 1 and 200")
			return
		}
		limit = value
	}
	changes, err := reader.ListChangeMarkers(request.Context(), limit)
	if err != nil {
		server.logger.Error("list changes failed", "error", err)
		writeError(writer, http.StatusServiceUnavailable, "storage backend unavailable")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"count": len(changes), "changes": changes})
}

func (server *Server) handleChangeAnalysis(writer http.ResponseWriter, request *http.Request) {
	reader, ok := server.reader.(storage.ChangeIntelligenceReader)
	if !ok {
		writeError(writer, http.StatusNotImplemented, "change intelligence is unavailable")
		return
	}
	analysis, found, err := reader.ChangeAnalysis(request.Context(), request.PathValue("id"))
	if err != nil {
		writeError(writer, http.StatusServiceUnavailable, "storage backend unavailable")
		return
	}
	if !found {
		writeError(writer, http.StatusNotFound, "change analysis has not been generated")
		return
	}
	writeJSON(writer, http.StatusOK, analysis)
}

func (server *Server) handleAnalyzeChange(writer http.ResponseWriter, request *http.Request) {
	reader, ok := server.reader.(storage.ChangeIntelligenceReader)
	if !ok {
		writeError(writer, http.StatusNotImplemented, "change intelligence is unavailable")
		return
	}
	before, err := analysisWindow(request.URL.Query().Get("before_seconds"), 900)
	if err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	after, err := analysisWindow(request.URL.Query().Get("after_seconds"), 900)
	if err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	analysis, err := reader.AnalyzeChange(request.Context(), request.PathValue("id"), before, after)
	if err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, analysis)
}

func (server *Server) handleIncidentChanges(writer http.ResponseWriter, request *http.Request) {
	dashboard, ok := server.reader.(storage.DashboardReader)
	if !ok {
		writeError(writer, http.StatusNotImplemented, "incident storage is unavailable")
		return
	}
	changes, ok := server.reader.(storage.ChangeIntelligenceReader)
	if !ok {
		writeError(writer, http.StatusNotImplemented, "change intelligence is unavailable")
		return
	}
	events, err := dashboard.IncidentEvents(request.Context(), request.PathValue("id"), 1000)
	if err != nil {
		writeError(writer, http.StatusServiceUnavailable, "storage backend unavailable")
		return
	}
	if len(events) == 0 {
		writeError(writer, http.StatusNotFound, "incident not found or contains no events")
		return
	}
	from, to := events[0].Timestamp, events[0].Timestamp
	for _, event := range events[1:] {
		if event.Timestamp.Before(from) {
			from = event.Timestamp
		}
		if event.Timestamp.After(to) {
			to = event.Timestamp
		}
	}
	result, err := changes.ChangesBetween(request.Context(), from.Add(-15*time.Minute), to.Add(5*time.Minute), 100)
	if err != nil {
		writeError(writer, http.StatusServiceUnavailable, "storage backend unavailable")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"incident_id": request.PathValue("id"), "count": len(result), "changes": result})
}

func validChangeStatus(status string) bool {
	switch status {
	case "started", "completed", "failed", "rolled_back", "changed":
		return true
	default:
		return false
	}
}

func changeEventType(kind, status string) (string, bool) {
	switch kind {
	case changeintel.KindDeployment:
		return "deployment." + status, true
	case changeintel.KindRollback:
		return "rollback." + status, true
	case changeintel.KindRelease:
		return "release." + status, true
	case changeintel.KindFeatureFlag:
		return "feature_flag.changed", true
	case changeintel.KindConfiguration:
		return "configuration.changed", true
	case changeintel.KindInfrastructure:
		return "infrastructure.changed", true
	default:
		return "", false
	}
}

func analysisWindow(raw string, fallback int) (time.Duration, error) {
	if raw == "" {
		return time.Duration(fallback) * time.Second, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 60 || value > 7200 {
		return 0, errors.New("analysis window must be between 60 and 7200 seconds")
	}
	return time.Duration(value) * time.Second, nil
}
