package api

import (
	"github.com/fuhrdan/TelemetryForge/internal/storage"
	"net/http"
)

func (s *Server) handleProofs(w http.ResponseWriter, r *http.Request) {
	rd, ok := s.reader.(storage.OperationalProofReader)
	if !ok {
		writeError(w, http.StatusNotImplemented, "operational proof history is unavailable")
		return
	}
	limit, ok := parseHistoryLimit(w, r)
	if !ok {
		return
	}
	items, err := rd.ListOperationalProofs(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "storage backend unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": len(items), "proofs": items})
}
