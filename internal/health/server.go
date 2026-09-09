// Package health provides the worker's Kubernetes-facing health endpoints.
package health

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

// Checker is a dependency readiness check.
type Checker interface {
	Ready(context.Context) error
}

// Server exposes liveness and dependency-aware readiness.
type Server struct {
	http   *http.Server
	logger *slog.Logger
	checks map[string]Checker
}

// New creates a worker health server.
func New(address string, logger *slog.Logger, checks map[string]Checker) *Server {
	mux := http.NewServeMux()
	server := &Server{
		http: &http.Server{
			Addr:              address,
			Handler:           mux,
			ReadHeaderTimeout: 2 * time.Second,
		},
		logger: logger,
		checks: checks,
	}
	mux.HandleFunc("GET /health", server.health)
	mux.HandleFunc("GET /ready", server.ready)
	return server
}

// Start serves until shutdown or a fatal listener error.
func (server *Server) Start() error {
	err := server.http.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

// Shutdown stops the health listener gracefully.
func (server *Server) Shutdown(ctx context.Context) error {
	return server.http.Shutdown(ctx)
}

func (server *Server) health(writer http.ResponseWriter, _ *http.Request) {
	write(writer, http.StatusOK, map[string]string{"status": "healthy"})
}

func (server *Server) ready(writer http.ResponseWriter, request *http.Request) {
	ctx, cancel := context.WithTimeout(request.Context(), 3*time.Second)
	defer cancel()

	for name, checker := range server.checks {
		if checker == nil {
			continue
		}
		if err := checker.Ready(ctx); err != nil {
			server.logger.Warn("worker not ready", "dependency", name, "error", err)
			write(writer, http.StatusServiceUnavailable, map[string]string{
				"status": "not_ready", "dependency": name,
			})
			return
		}
	}
	write(writer, http.StatusOK, map[string]string{"status": "ready"})
}

func write(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}
