// Command gateway starts the TelemetryForge HTTP ingestion gateway.
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/fuhrdan/TelemetryForge/internal/api"
	"github.com/fuhrdan/TelemetryForge/internal/config"
	"github.com/fuhrdan/TelemetryForge/internal/logging"
)

func main() {
	logger := logging.New()
	cfg := config.Load()

	handler := api.NewServer(logger)
	httpServer := &http.Server{
		Addr:         cfg.Address,
		Handler:      handler,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}

	errorChannel := make(chan error, 1)
	go func() {
		logger.Info("TelemetryForge gateway starting", "address", cfg.Address)
		errorChannel <- httpServer.ListenAndServe()
	}()

	signalContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case <-signalContext.Done():
		logger.Info("shutdown signal received")
	case err := <-errorChannel:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("gateway stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	}

	shutdownContext, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := httpServer.Shutdown(shutdownContext); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}

	logger.Info("TelemetryForge gateway stopped")
}
