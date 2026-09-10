// Command gateway starts the TelemetryForge HTTP ingestion gateway.
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/api"
	"github.com/fuhrdan/TelemetryForge/internal/config"
	"github.com/fuhrdan/TelemetryForge/internal/logging"
	"github.com/fuhrdan/TelemetryForge/internal/observability"
	"github.com/fuhrdan/TelemetryForge/internal/security"
	"github.com/fuhrdan/TelemetryForge/internal/storage"
	"github.com/fuhrdan/TelemetryForge/internal/stream"
)

func main() {
	logger := logging.New()
	cfg := config.Load()
	metrics := observability.NewMetrics("gateway")
	traceShutdown, err := observability.InitTracing(
		context.Background(),
		"telemetryforge-gateway",
		"1.7.0",
		os.Getenv("TELEMETRYFORGE_OTLP_TRACES_ENDPOINT"),
	)
	if err != nil {
		logger.Error("OpenTelemetry initialization failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = traceShutdown(shutdownCtx)
	}()

	var authenticator *security.Authenticator
	switch cfg.AuthMode {
	case "", "disabled":
		authenticator = security.Disabled(cfg.DefaultTenant)
	case "api_key":
		authenticator, err = security.LoadAPIKeys(cfg.APIKeysFile)
		if err != nil {
			logger.Error("API key authentication initialization failed", "error", err)
			os.Exit(1)
		}
	default:
		logger.Error("unsupported authentication mode", "mode", cfg.AuthMode)
		os.Exit(1)
	}

	publisher, err := stream.NewKafkaPublisher(stream.KafkaConfig{
		Brokers:        cfg.KafkaBrokers,
		ClientID:       cfg.KafkaClientID,
		ProduceTimeout: cfg.KafkaTimeout,
		Security:       stream.KafkaSecurityFromEnv(),
	}, logger)
	if err != nil {
		logger.Error("Kafka publisher initialization failed", "error", err)
		os.Exit(1)
	}
	defer publisher.Close()

	store, err := storage.NewPostgresStore(context.Background(), cfg.DatabaseURL)
	if err != nil {
		logger.Error("PostgreSQL store initialization failed", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	apiHandler := api.NewServerWithObserver(logger, publisher, api.Topics{
		Raw:    cfg.KafkaRawTopic,
		Metric: cfg.KafkaMetricTopic,
	}, store, metrics)
	apiHandler.SetRedactor(security.NewRedactor(cfg.RedactTags, cfg.RedactPayload))
	root := http.NewServeMux()
	root.Handle("GET /metrics", metrics.Handler())
	root.Handle("/", apiHandler)

	httpServer := &http.Server{
		Addr:         cfg.Address,
		Handler:      authenticator.Middleware(metrics.Middleware(root)),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}

	errorChannel := make(chan error, 1)
	go func() {
		logger.Info(
			"TelemetryForge gateway starting",
			"address", cfg.Address,
			"kafka_brokers", cfg.KafkaBrokers,
			"raw_topic", cfg.KafkaRawTopic,
			"metric_topic", cfg.KafkaMetricTopic,
			"auth_mode", cfg.AuthMode,
		)
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
