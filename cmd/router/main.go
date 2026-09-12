// Command router drains the durable TelemetryForge routing outbox.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/health"
	"github.com/fuhrdan/TelemetryForge/internal/logging"
	"github.com/fuhrdan/TelemetryForge/internal/observability"
	"github.com/fuhrdan/TelemetryForge/internal/router"
	"github.com/fuhrdan/TelemetryForge/internal/security"
	"github.com/fuhrdan/TelemetryForge/internal/storage"
	"github.com/fuhrdan/TelemetryForge/internal/stream"
)

func main() {
	logger := logging.New()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	metrics := observability.NewMetrics("router")
	traceShutdown, err := observability.InitTracing(
		ctx, "telemetryforge-router", "2.3.0", os.Getenv("TELEMETRYFORGE_OTLP_TRACES_ENDPOINT"),
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

	databaseURL := env("TELEMETRYFORGE_DATABASE_URL", "postgres://telemetryforge:telemetryforge@localhost:5432/telemetryforge?sslmode=disable")
	store, err := storage.NewPostgresStore(ctx, databaseURL)
	if err != nil {
		logger.Error("create PostgreSQL store", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	routingPath := env("TELEMETRYFORGE_ROUTING_FILE", "routing/active.json")
	routingConfig, err := router.Load(routingPath)
	if err != nil {
		logger.Error("load routing config", "path", routingPath, "error", err)
		os.Exit(1)
	}

	dispatcher, err := router.NewDispatcher(
		routingConfig,
		store,
		router.SenderFactoryConfig{
			KafkaBrokers:  splitCSV(env("TELEMETRYFORGE_KAFKA_BROKERS", "localhost:9092")),
			KafkaSecurity: stream.KafkaSecurityFromEnv(),
			Logger:        logger,
		},
		logger,
		metrics,
	)
	if err != nil {
		logger.Error("create routing dispatcher", "error", err)
		os.Exit(1)
	}
	defer dispatcher.Close()

	adminAddress := env("TELEMETRYFORGE_ROUTER_ADMIN_ADDRESS", ":8082")
	metricsHandler := http.Handler(metrics.Handler())
	if strings.EqualFold(env("TELEMETRYFORGE_AUTH_MODE", "disabled"), "api_key") {
		authenticator, err := security.LoadAPIKeys(env("TELEMETRYFORGE_API_KEYS_FILE", "security/api-keys.example.json"))
		if err != nil {
			logger.Error("router metrics authentication initialization failed", "error", err)
			os.Exit(1)
		}
		metricsHandler = authenticator.Middleware(metricsHandler)
	}

	healthServer := health.New(adminAddress, logger, map[string]health.Checker{
		"database": store,
		"router":   dispatcher,
	}, metricsHandler)
	go func() {
		if err := healthServer.Start(); err != nil {
			logger.Error("router health server stopped", "error", err)
			stop()
		}
	}()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = healthServer.Shutdown(shutdownCtx)
	}()

	instanceID := env("TELEMETRYFORGE_ROUTER_INSTANCE_ID", defaultInstanceID())
	heartbeat := func() {
		states := dispatcher.ConnectorRuntimeStates(ctx, instanceID)
		for _, state := range states {
			metrics.ConnectorReady(state.Destination, state.Kind, state.Ready)
			if err := store.RecordConnectorRuntimeState(ctx, state); err != nil {
				logger.Warn("record connector runtime heartbeat", "destination", state.Destination, "error", err)
			}
		}
	}
	heartbeat()
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		pruneTicker := time.NewTicker(time.Hour)
		defer pruneTicker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				heartbeat()
			case <-pruneTicker.C:
				if pruned, err := store.PruneConnectorRuntimeStates(ctx, time.Now().UTC().Add(-24*time.Hour)); err != nil {
					logger.Warn("prune stale connector runtime state", "error", err)
				} else if pruned > 0 {
					logger.Info("pruned stale connector runtime state", "rows", pruned)
				}
			}
		}
	}()

	logger.Info("TelemetryForge router started",
		"config", routingConfig.Name+"@"+routingConfig.Version,
		"destinations", len(routingConfig.EnabledDestinations()),
		"instance_id", instanceID,
		"admin_address", adminAddress)

	dispatcher.Run(ctx)
	logger.Info("TelemetryForge router stopped")
}

func env(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			result = append(result, part)
		}
	}
	return result
}

func defaultInstanceID() string {
	if hostname, err := os.Hostname(); err == nil && strings.TrimSpace(hostname) != "" {
		return hostname
	}
	return fmt.Sprintf("router-%d", time.Now().UTC().UnixNano())
}
