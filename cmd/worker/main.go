// Command worker runs the TelemetryForge stream-processing consumer.
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/health"
	"github.com/fuhrdan/TelemetryForge/internal/incident"
	"github.com/fuhrdan/TelemetryForge/internal/logging"
	"github.com/fuhrdan/TelemetryForge/internal/observability"
	"github.com/fuhrdan/TelemetryForge/internal/policy"
	"github.com/fuhrdan/TelemetryForge/internal/router"
	"github.com/fuhrdan/TelemetryForge/internal/security"
	"github.com/fuhrdan/TelemetryForge/internal/shaping"
	"github.com/fuhrdan/TelemetryForge/internal/storage"
	"github.com/fuhrdan/TelemetryForge/internal/stream"
	"github.com/fuhrdan/TelemetryForge/internal/worker"
)

func main() {
	logger := logging.New()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	metrics := observability.NewMetrics("worker")
	traceShutdown, err := observability.InitTracing(
		ctx,
		"telemetryforge-worker",
		"1.5.0",
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

	brokers := splitCSV(env("TELEMETRYFORGE_KAFKA_BROKERS", "localhost:9092"))
	topics := splitCSV(env("TELEMETRYFORGE_WORKER_TOPICS", "telemetry.raw,telemetry.metrics"))
	workers := envInt("TELEMETRYFORGE_WORKER_COUNT", 4)
	queueCapacity := envInt("TELEMETRYFORGE_WORKER_QUEUE_CAPACITY", 256)
	databaseURL := env("TELEMETRYFORGE_DATABASE_URL",
		"postgres://telemetryforge:telemetryforge@localhost:5432/telemetryforge?sslmode=disable")

	store, err := storage.NewPostgresStore(ctx, databaseURL)
	if err != nil {
		logger.Error("create PostgreSQL store", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	activePolicyPath := env("TELEMETRYFORGE_POLICY_FILE", "policies/active.json")
	activePolicy, err := policy.Load(activePolicyPath)
	if err != nil {
		logger.Error("load active telemetry policy", "path", activePolicyPath, "error", err)
		os.Exit(1)
	}

	var shadowPolicy *policy.Policy
	shadowPolicyPath := env("TELEMETRYFORGE_SHADOW_POLICY_FILE", "policies/shadow.json")
	if strings.EqualFold(shadowPolicyPath, "disabled") {
		shadowPolicyPath = ""
	}
	if shadowPolicyPath != "" {
		loadedShadow, err := policy.Load(shadowPolicyPath)
		if err != nil {
			logger.Error("load shadow telemetry policy", "path", shadowPolicyPath, "error", err)
			os.Exit(1)
		}
		shadowPolicy = &loadedShadow
	}

	routingPath := env("TELEMETRYFORGE_ROUTING_FILE", "routing/active.json")
	activeRouting, err := router.Load(routingPath)
	if err != nil {
		logger.Error("load active routing config", "path", routingPath, "error", err)
		os.Exit(1)
	}

	var shadowRouting *router.Config
	shadowRoutingPath := env("TELEMETRYFORGE_SHADOW_ROUTING_FILE", "routing/shadow.json")
	if strings.EqualFold(shadowRoutingPath, "disabled") {
		shadowRoutingPath = ""
	}
	if shadowRoutingPath != "" {
		loadedRouting, err := router.Load(shadowRoutingPath)
		if err != nil {
			logger.Error("load shadow routing config", "path", shadowRoutingPath, "error", err)
			os.Exit(1)
		}
		shadowRouting = &loadedRouting
	}
	routingPlanner, err := router.NewPlanner(activeRouting, shadowRouting, store)
	if err != nil {
		logger.Error("create routing planner", "error", err)
		os.Exit(1)
	}
	routingProcessor, err := worker.NewRouterPlanner(routingPlanner)
	if err != nil {
		logger.Error("create routing processor", "error", err)
		os.Exit(1)
	}

	activeShapingPath := env("TELEMETRYFORGE_SHAPING_FILE", "shaping/active.json")
	activeShaping, err := shaping.Load(activeShapingPath)
	if err != nil {
		logger.Error("load active shaping config", "path", activeShapingPath, "error", err)
		os.Exit(1)
	}

	var shadowShaping *shaping.Config
	shadowShapingPath := env("TELEMETRYFORGE_SHADOW_SHAPING_FILE", "shaping/shadow.json")
	if strings.EqualFold(shadowShapingPath, "disabled") {
		shadowShapingPath = ""
	}
	if shadowShapingPath != "" {
		loadedShaping, loadErr := shaping.Load(shadowShapingPath)
		if loadErr != nil {
			logger.Error("load shadow shaping config", "path", shadowShapingPath, "error", loadErr)
			os.Exit(1)
		}
		shadowShaping = &loadedShaping
	}
	shapingEngine, err := shaping.NewEngine(activeShaping, shadowShaping)
	if err != nil {
		logger.Error("create shaping engine", "error", err)
		os.Exit(1)
	}
	pressureController := shaping.NewPressureController()
	shapingProcessor, err := worker.NewAdaptiveShaper(shapingEngine, store, pressureController, logger, metrics)
	if err != nil {
		logger.Error("create adaptive shaping processor", "error", err)
		os.Exit(1)
	}

	activeTracker := storage.NewDistributedCardinalityTracker(store, "active")
	var shadowTracker policy.CardinalityTracker
	if shadowPolicy != nil {
		shadowTracker = storage.NewDistributedCardinalityTracker(store, "shadow")
	}
	policyEngine, err := policy.NewEngineWithTrackers(
		activePolicy,
		shadowPolicy,
		store,
		activeTracker,
		shadowTracker,
		envInt("TELEMETRYFORGE_CARDINALITY_MAX_DIMENSIONS", 20000),
	)
	if err != nil {
		logger.Error("create policy engine", "error", err)
		os.Exit(1)
	}

	recorder, err := worker.NewFlightRecorder(store)
	if err != nil {
		logger.Error("create flight recorder", "error", err)
		os.Exit(1)
	}

	persister, err := worker.NewPersister(store)
	if err != nil {
		logger.Error("create persistence processor", "error", err)
		os.Exit(1)
	}

	// Preserve the incoming envelope before normalization. That ordering is
	// deliberate: incident captures should show what the pipeline actually saw.
	incidentConfig := incident.DefaultConfig()
	incidentConfig.LatencyThresholdMS = envFloat("TELEMETRYFORGE_INCIDENT_LATENCY_MS", incidentConfig.LatencyThresholdMS)
	incidentConfig.ErrorThreshold = envInt("TELEMETRYFORGE_INCIDENT_ERROR_COUNT", incidentConfig.ErrorThreshold)
	detector := incident.NewDetector(store, incidentConfig)

	schemaInspector := worker.NewSchemaInspectorWithMode(
		store,
		logger,
		envBool("TELEMETRYFORGE_SCHEMA_FAIL_OPEN", true),
	)

	pipeline, err := worker.NewChain(
		recorder,
		worker.Normalizer{},
		schemaInspector,
		worker.NewPolicyProcessor(policyEngine),
		shapingProcessor,
		persister,
		routingProcessor,
		worker.NewIncidentDetector(detector, logger),
	)
	if err != nil {
		logger.Error("create processing pipeline", "error", err)
		os.Exit(1)
	}

	consumer, err := stream.NewKafkaConsumer(stream.ConsumerConfig{
		Brokers:  brokers,
		ClientID: env("TELEMETRYFORGE_WORKER_CLIENT_ID", "telemetryforge-worker"),
		GroupID:  env("TELEMETRYFORGE_WORKER_GROUP_ID", "telemetryforge-processors"),
		Topics:   topics,
		DLQTopic: env("TELEMETRYFORGE_WORKER_DLQ_TOPIC", "telemetry.dlq"),
		Observer: metrics,
		Security: stream.KafkaSecurityFromEnv(),
	}, logger)
	if err != nil {
		logger.Error("create Kafka consumer", "error", err)
		os.Exit(1)
	}
	defer consumer.Close()

	adminAddress := env("TELEMETRYFORGE_WORKER_ADMIN_ADDRESS", ":8081")
	metricsHandler := http.Handler(metrics.Handler())
	if strings.EqualFold(env("TELEMETRYFORGE_AUTH_MODE", "disabled"), "api_key") {
		authenticator, err := security.LoadAPIKeys(
			env("TELEMETRYFORGE_API_KEYS_FILE", "security/api-keys.example.json"),
		)
		if err != nil {
			logger.Error("worker metrics authentication initialization failed", "error", err)
			os.Exit(1)
		}
		metricsHandler = authenticator.Middleware(metricsHandler)
	}

	healthServer := health.New(adminAddress, logger, map[string]health.Checker{
		"kafka":    consumer,
		"database": store,
	}, metricsHandler)
	go func() {
		if err := healthServer.Start(); err != nil {
			logger.Error("worker health server stopped", "error", err)
			stop()
		}
	}()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = healthServer.Shutdown(shutdownCtx)
	}()

	go consumer.RunLagMonitor(ctx, 15*time.Second)

	poolObserver := worker.NewCompositeObserver(metrics, pressureController)
	pool, err := worker.NewPoolWithObserver(workers, queueCapacity, pipeline, logger, poolObserver)
	if err != nil {
		logger.Error("create worker pool", "error", err)
		os.Exit(1)
	}
	pool.Start(ctx)

	logger.Info("TelemetryForge worker started",
		"workers", workers,
		"queue_capacity", queueCapacity,
		"group_id", env("TELEMETRYFORGE_WORKER_GROUP_ID", "telemetryforge-processors"),
		"topics", strings.Join(topics, ","),
		"dlq_topic", env("TELEMETRYFORGE_WORKER_DLQ_TOPIC", "telemetry.dlq"),
		"persistence", "postgresql/timescaledb",
		"flight_recorder", "enabled",
		"schema_intelligence", "enabled",
		"distributed_cardinality", "timescaledb-hourly",
		"telemetry_router", activeRouting.Name+"@"+activeRouting.Version,
		"shadow_routing", shadowRouting != nil,
		"adaptive_shaping", activeShaping.Name+"@"+activeShaping.Version,
		"shadow_shaping", shadowShaping != nil,
		"policy", activePolicy.Name+"@"+activePolicy.Version,
		"shadow_policy", shadowPolicyPath != "",
		"admin_address", adminAddress)

	if err := consumer.Run(ctx, pool); err != nil {
		logger.Error("consumer stopped with error", "error", err)
	}
	pool.Close()
	logger.Info("TelemetryForge worker stopped")
}

func env(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func envInt(name string, fallback int) int {
	value, err := strconv.Atoi(env(name, ""))
	if err != nil || value < 1 {
		return fallback
	}
	return value
}

func envFloat(name string, fallback float64) float64 {
	value, err := strconv.ParseFloat(env(name, ""), 64)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
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

func envBool(name string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(name)))
	if value == "" {
		return fallback
	}
	switch value {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}
