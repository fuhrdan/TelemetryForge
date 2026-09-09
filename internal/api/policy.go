package api

import (
	"net/http"
	"strconv"

	"github.com/fuhrdan/TelemetryForge/internal/storage"
)

func (server *Server) handleCardinalityFindings(writer http.ResponseWriter, request *http.Request) {
	reader, ok := server.reader.(storage.PolicyReader)
	if !ok {
		writeError(writer, http.StatusNotImplemented, "policy storage is unavailable")
		return
	}
	limit, ok := parsePolicyLimit(writer, request)
	if !ok {
		return
	}

	findings, err := reader.ListCardinalityFindings(request.Context(), limit)
	if err != nil {
		server.logger.Error("cardinality finding query failed", "error", err)
		writeError(writer, http.StatusServiceUnavailable, "storage backend unavailable")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"count":    len(findings),
		"findings": findings,
	})
}

func (server *Server) handlePolicyDiffs(writer http.ResponseWriter, request *http.Request) {
	reader, ok := server.reader.(storage.PolicyReader)
	if !ok {
		writeError(writer, http.StatusNotImplemented, "policy storage is unavailable")
		return
	}
	limit, ok := parsePolicyLimit(writer, request)
	if !ok {
		return
	}

	diffs, err := reader.ListPolicyDiffs(request.Context(), limit)
	if err != nil {
		server.logger.Error("policy shadow-diff query failed", "error", err)
		writeError(writer, http.StatusServiceUnavailable, "storage backend unavailable")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"count": len(diffs),
		"diffs": diffs,
	})
}

func parsePolicyLimit(writer http.ResponseWriter, request *http.Request) (int, bool) {
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

func (server *Server) handleCardinalityStates(writer http.ResponseWriter, request *http.Request) {
	reader, ok := server.reader.(storage.PolicyReader)
	if !ok {
		writeError(writer, http.StatusNotImplemented, "policy storage is unavailable")
		return
	}
	limit, ok := parsePolicyLimit(writer, request)
	if !ok {
		return
	}
	mode := request.URL.Query().Get("mode")
	if mode == "" {
		mode = "active"
	}
	if mode != "active" && mode != "shadow" {
		writeError(writer, http.StatusBadRequest, "mode must be active or shadow")
		return
	}

	states, err := reader.ListDistributedCardinalityStates(request.Context(), mode, limit)
	if err != nil {
		server.logger.Error("distributed cardinality query failed", "mode", mode, "error", err)
		writeError(writer, http.StatusServiceUnavailable, "storage backend unavailable")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"count":  len(states),
		"mode":   mode,
		"window": "1h",
		"states": states,
	})
}

func (server *Server) handleCardinalityBudgets(writer http.ResponseWriter, request *http.Request) {
	reader, ok := server.reader.(storage.PolicyReader)
	if !ok {
		writeError(writer, http.StatusNotImplemented, "policy storage is unavailable")
		return
	}
	limit, ok := parsePolicyLimit(writer, request)
	if !ok {
		return
	}

	budgets, err := reader.ListCardinalityBudgetStatus(request.Context(), limit)
	if err != nil {
		server.logger.Error("cardinality budget query failed", "error", err)
		writeError(writer, http.StatusServiceUnavailable, "storage backend unavailable")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"count":   len(budgets),
		"budgets": budgets,
	})
}
