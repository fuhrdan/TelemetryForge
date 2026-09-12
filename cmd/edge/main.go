// Command edge starts the TelemetryForge durable edge ingestion service.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/api"
	"github.com/fuhrdan/TelemetryForge/internal/config"
	"github.com/fuhrdan/TelemetryForge/internal/edge"
	"github.com/fuhrdan/TelemetryForge/internal/logging"
	"github.com/fuhrdan/TelemetryForge/internal/observability"
	"github.com/fuhrdan/TelemetryForge/internal/security"
	"github.com/fuhrdan/TelemetryForge/internal/stream"
	"github.com/fuhrdan/TelemetryForge/internal/wal"
)

const version = "2.1.0"

func main() {
	logger := logging.New()
	cfg := config.Load()
	metrics := observability.NewMetrics("edge")
	traceShutdown, err := observability.InitTracing(context.Background(), "telemetryforge-edge", version, os.Getenv("TELEMETRYFORGE_OTLP_TRACES_ENDPOINT"))
	if err != nil {
		logger.Error("OpenTelemetry initialization failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = traceShutdown(ctx)
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

	edgeID := envOrDefault("TELEMETRYFORGE_EDGE_ID", hostnameOrDefault("edge-local"))
	walStore, err := wal.Open(wal.Config{Directory: envOrDefault("TELEMETRYFORGE_EDGE_WAL_DIR", "data/edge-wal"), EdgeID: edgeID, SegmentSizeBytes: int64Env("TELEMETRYFORGE_EDGE_WAL_SEGMENT_BYTES", 64<<20), MaxBytes: int64Env("TELEMETRYFORGE_EDGE_WAL_MAX_BYTES", 4<<30)})
	if err != nil {
		logger.Error("edge WAL initialization failed", "error", err)
		os.Exit(1)
	}
	kafkaPublisher, err := stream.NewKafkaPublisher(stream.KafkaConfig{Brokers: cfg.KafkaBrokers, ClientID: envOrDefault("TELEMETRYFORGE_EDGE_KAFKA_CLIENT_ID", "telemetryforge-edge-"+edgeID), ProduceTimeout: cfg.KafkaTimeout, Security: stream.KafkaSecurityFromEnv()}, logger)
	if err != nil {
		_ = walStore.Close()
		logger.Error("Kafka publisher initialization failed", "error", err)
		os.Exit(1)
	}
	durablePublisher := edge.NewPublisher(walStore, kafkaPublisher, logger)
	defer durablePublisher.Close()
	apiHandler := api.NewServerWithObserver(logger, durablePublisher, api.Topics{Raw: cfg.KafkaRawTopic, Metric: cfg.KafkaMetricTopic}, nil, metrics)
	apiHandler.SetRedactor(security.NewRedactor(cfg.RedactTags, cfg.RedactPayload))
	root := http.NewServeMux()
	root.Handle("GET /metrics", metrics.Handler())
	root.HandleFunc("GET /edge/status", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(durablePublisher.Stats())
	})
	root.Handle("/", apiHandler)
	address := envOrDefault("TELEMETRYFORGE_EDGE_ADDRESS", ":8083")
	httpServer := &http.Server{Addr: address, Handler: authenticator.Middleware(metrics.Middleware(root)), ReadTimeout: cfg.ReadTimeout, WriteTimeout: cfg.WriteTimeout}
	errorChannel := make(chan error, 1)
	go func() {
		stats := durablePublisher.Stats()
		logger.Info("TelemetryForge durable edge starting", "version", version, "edge_id", edgeID, "address", address, "wal_directory", stats.Directory, "wal_max_bytes", stats.MaxBytes, "pending_records", stats.PendingRecords)
		errorChannel <- httpServer.ListenAndServe()
	}()
	signalContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case <-signalContext.Done():
		logger.Info("shutdown signal received")
	case err := <-errorChannel:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("edge stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	}
	shutdownContext, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := httpServer.Shutdown(shutdownContext); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
}

func envOrDefault(name, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}
func int64Env(name string, fallback int64) int64 {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
func hostnameOrDefault(fallback string) string {
	hostname, err := os.Hostname()
	if err != nil || strings.TrimSpace(hostname) == "" {
		return fallback
	}
	return hostname
}
