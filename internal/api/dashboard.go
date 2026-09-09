package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/storage"
)

const livePollInterval = time.Second

func (server *Server) handleSummary(writer http.ResponseWriter, request *http.Request) {
	reader, ok := server.reader.(storage.DashboardReader)
	if !ok {
		writeError(writer, http.StatusNotImplemented, "dashboard storage is unavailable")
		return
	}

	window := 5 * time.Minute
	if raw := request.URL.Query().Get("window"); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil || parsed < time.Minute || parsed > 24*time.Hour {
			writeError(writer, http.StatusBadRequest, "window must be a duration between 1m and 24h")
			return
		}
		window = parsed
	}

	summary, err := reader.Summary(request.Context(), window)
	if err != nil {
		server.logger.Error("dashboard summary failed", "error", err)
		writeError(writer, http.StatusServiceUnavailable, "storage backend unavailable")
		return
	}
	writeJSON(writer, http.StatusOK, summary)
}

func (server *Server) handleIncidents(writer http.ResponseWriter, request *http.Request) {
	reader, ok := server.reader.(storage.DashboardReader)
	if !ok {
		writeError(writer, http.StatusNotImplemented, "incident storage is unavailable")
		return
	}

	limit := 25
	if raw := request.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 100 {
			writeError(writer, http.StatusBadRequest, "limit must be between 1 and 100")
			return
		}
		limit = parsed
	}

	incidents, err := reader.ListIncidents(request.Context(), limit)
	if err != nil {
		server.logger.Error("incident query failed", "error", err)
		writeError(writer, http.StatusServiceUnavailable, "storage backend unavailable")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"count":     len(incidents),
		"incidents": incidents,
	})
}

func (server *Server) handleIncidentEvents(writer http.ResponseWriter, request *http.Request) {
	reader, ok := server.reader.(storage.DashboardReader)
	if !ok {
		writeError(writer, http.StatusNotImplemented, "incident storage is unavailable")
		return
	}

	incidentID := request.PathValue("id")
	if incidentID == "" {
		writeError(writer, http.StatusBadRequest, "incident id is required")
		return
	}

	events, err := reader.IncidentEvents(request.Context(), incidentID, 500)
	if err != nil {
		server.logger.Error("incident event query failed", "incident_id", incidentID, "error", err)
		writeError(writer, http.StatusServiceUnavailable, "storage backend unavailable")
		return
	}

	writeJSON(writer, http.StatusOK, map[string]any{
		"incident_id": incidentID,
		"count":       len(events),
		"events":      events,
	})
}

// handleLive streams newly persisted telemetry using Server-Sent Events.
//
// v0.6.0 deliberately reads durable shared storage rather than an in-memory
// process bus. A timestamp+event-ID cursor prevents high-volume batches from
// skipping records that share the same event timestamp.
func (server *Server) handleLive(writer http.ResponseWriter, request *http.Request) {
	flusher, ok := writer.(http.Flusher)
	if !ok {
		writeError(writer, http.StatusInternalServerError, "streaming is not supported")
		return
	}

	reader, ok := server.reader.(storage.DashboardReader)
	if !ok {
		writeError(writer, http.StatusNotImplemented, "live storage is unavailable")
		return
	}

	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Connection", "keep-alive")
	writer.Header().Set("X-Accel-Buffering", "no")

	cursorTime := time.Now().UTC().Add(-5 * time.Second)
	cursorID := ""
	if lastEventID := request.Header.Get("Last-Event-ID"); lastEventID != "" {
		parts := strings.SplitN(lastEventID, "|", 2)
		if len(parts) == 2 {
			if parsed, err := time.Parse(time.RFC3339Nano, parts[0]); err == nil {
				cursorTime = parsed
				cursorID = parts[1]
			}
		}
	}
	ticker := time.NewTicker(livePollInterval)
	heartbeat := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	defer heartbeat.Stop()

	fmt.Fprintf(writer, "event: ready\ndata: {\"status\":\"connected\"}\n\n")
	flusher.Flush()

	for {
		select {
		case <-request.Context().Done():
			return

		case <-heartbeat.C:
			fmt.Fprint(writer, ": heartbeat\n\n")
			flusher.Flush()

		case <-ticker.C:
			// Drain more than one batch before sleeping again. This avoids
			// falling permanently behind when a one-second interval contains
			// more events than one query limit.
			for batch := 0; batch < 20; batch++ {
				events, err := reader.LiveEvents(request.Context(), cursorTime, cursorID, 500)
				if err != nil {
					server.logger.Warn("live telemetry poll failed", "error", err)
					break
				}
				if len(events) == 0 {
					break
				}

				for _, record := range events {
					payload, err := json.Marshal(record.Event)
					if err != nil {
						continue
					}
					fmt.Fprintf(
						writer,
						"event: telemetry\nid: %s|%s\ndata: %s\n\n",
						record.IngestedAt.UTC().Format(time.RFC3339Nano),
						record.Event.ID,
						payload,
					)
					cursorTime = record.IngestedAt
					cursorID = record.Event.ID
				}
				flusher.Flush()

				if len(events) < 500 {
					break
				}
			}
		}
	}
}
