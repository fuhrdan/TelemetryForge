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
	"github.com/fuhrdan/TelemetryForge/internal/mesh"
	"github.com/fuhrdan/TelemetryForge/internal/observability"
	"github.com/fuhrdan/TelemetryForge/internal/replication"
	"github.com/fuhrdan/TelemetryForge/internal/security"
	"github.com/fuhrdan/TelemetryForge/internal/stream"
	"github.com/fuhrdan/TelemetryForge/internal/wal"
)

const version = "2.3.0"

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
	localNode := replication.Node{
		ID: edgeID,
		Domain: replication.FailureDomain{
			Cloud:  strings.TrimSpace(os.Getenv("TELEMETRYFORGE_EDGE_CLOUD")),
			Region: strings.TrimSpace(os.Getenv("TELEMETRYFORGE_EDGE_REGION")),
			Zone:   strings.TrimSpace(os.Getenv("TELEMETRYFORGE_EDGE_ZONE")),
		},
	}
	peers, err := replication.ParsePeers(os.Getenv("TELEMETRYFORGE_EDGE_PEERS"))
	if err != nil {
		logger.Error("edge replication peer configuration invalid", "error", err)
		os.Exit(1)
	}
	replicationManager, err := replication.NewManager(replication.Config{
		Local:   localNode,
		Peers:   peers,
		Mode:    replication.ParseMode(os.Getenv("TELEMETRYFORGE_EDGE_DURABILITY_MODE")),
		Quorum:  replication.ParseQuorum(os.Getenv("TELEMETRYFORGE_EDGE_REPLICATION_QUORUM")),
		Timeout: replication.ParseTimeout(os.Getenv("TELEMETRYFORGE_EDGE_REPLICATION_TIMEOUT"), 3*time.Second),
		Token:   strings.TrimSpace(os.Getenv("TELEMETRYFORGE_EDGE_REPLICATION_TOKEN")),
	})
	if err != nil {
		logger.Error("edge replication configuration invalid", "error", err)
		os.Exit(1)
	}
	replicaStore, err := replication.OpenStoreWithConfig(replication.StoreConfig{Directory: envOrDefault("TELEMETRYFORGE_EDGE_REPLICA_DIR", "data/edge-replicas"), MaxBytes: int64Env("TELEMETRYFORGE_EDGE_REPLICA_MAX_BYTES", 8<<30)})
	if err != nil {
		logger.Error("edge replica store initialization failed", "error", err)
		os.Exit(1)
	}
	defer replicaStore.Close()

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

	meshLocal := mesh.Node{
		ID: edgeID,
		Domain: mesh.FailureDomain{
			Cloud:  envOrDefault("TELEMETRYFORGE_MESH_CLOUD", localNode.Domain.Cloud),
			Region: envOrDefault("TELEMETRYFORGE_MESH_REGION", localNode.Domain.Region),
			Zone:   envOrDefault("TELEMETRYFORGE_MESH_ZONE", localNode.Domain.Zone),
		},
	}
	meshPeers, err := mesh.ParsePeers(os.Getenv("TELEMETRYFORGE_MESH_PEERS"))
	if err != nil {
		kafkaPublisher.Close()
		_ = walStore.Close()
		logger.Error("edge mesh peer configuration invalid", "error", err)
		os.Exit(1)
	}
	meshState := func(ctx context.Context) mesh.State {
		stats := walStore.Stats()
		pressure := 0.0
		if stats.MaxBytes > 0 {
			pressure = float64(stats.Bytes) / float64(stats.MaxBytes)
		}
		readyContext, cancel := context.WithTimeout(ctx, time.Second)
		ready := kafkaPublisher.Ready(readyContext) == nil
		cancel()
		return mesh.State{
			Node:           meshLocal,
			Ready:          ready,
			Draining:       boolEnv("TELEMETRYFORGE_MESH_DRAINING", false),
			PendingRecords: stats.PendingRecords,
			WALBytes:       stats.Bytes,
			WALMaxBytes:    stats.MaxBytes,
			Pressure:       pressure,
			ObservedAt:     time.Now().UTC(),
		}
	}
	meshManager, err := mesh.NewManager(mesh.Config{
		Local:         meshLocal,
		Peers:         meshPeers,
		Token:         strings.TrimSpace(os.Getenv("TELEMETRYFORGE_MESH_TOKEN")),
		Policy:        envOrDefault("TELEMETRYFORGE_MESH_POLICY", "locality"),
		ProbeInterval: mesh.ParseDuration(os.Getenv("TELEMETRYFORGE_MESH_PROBE_INTERVAL"), 5*time.Second),
		Timeout:       mesh.ParseDuration(os.Getenv("TELEMETRYFORGE_MESH_TIMEOUT"), 2*time.Second),
		StaleAfter:    mesh.ParseDuration(os.Getenv("TELEMETRYFORGE_MESH_STALE_AFTER"), 15*time.Second),
		MaxPressure:   mesh.ParsePressure(os.Getenv("TELEMETRYFORGE_MESH_MAX_PRESSURE"), 0.90),
	}, meshState)
	if err != nil {
		kafkaPublisher.Close()
		_ = walStore.Close()
		logger.Error("edge mesh configuration invalid", "error", err)
		os.Exit(1)
	}
	meshContext, stopMesh := context.WithCancel(context.Background())
	defer stopMesh()
	go meshManager.Run(meshContext)
	meshPublisher := mesh.NewPublisher(meshManager, kafkaPublisher)
	durablePublisher := edge.NewPublisher(walStore, meshPublisher, logger, replicationManager)
	defer durablePublisher.Close()

	apiHandler := api.NewServerWithObserver(logger, durablePublisher, api.Topics{Raw: cfg.KafkaRawTopic, Metric: cfg.KafkaMetricTopic}, nil, metrics)
	apiHandler.SetRedactor(security.NewRedactor(cfg.RedactTags, cfg.RedactPayload))

	publicMux := http.NewServeMux()
	publicMux.Handle("GET /metrics", metrics.Handler())
	publicMux.HandleFunc("GET /edge/status", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(struct {
			edge.Status
			ReplicaStore replication.StoreStats `json:"replica_store"`
			Mesh         mesh.Snapshot          `json:"mesh"`
		}{Status: durablePublisher.Stats(), ReplicaStore: replicaStore.Stats(), Mesh: meshManager.Snapshot(request.Context())})
	})
	publicMux.HandleFunc("GET /edge/route", func(writer http.ResponseWriter, request *http.Request) {
		key := strings.TrimSpace(request.URL.Query().Get("key"))
		if key == "" {
			http.Error(writer, "key query parameter is required", http.StatusBadRequest)
			return
		}
		route, routeErr := meshManager.Route(request.Context(), key)
		if routeErr != nil {
			http.Error(writer, routeErr.Error(), http.StatusServiceUnavailable)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(route)
	})
	publicMux.Handle("/", apiHandler)

	// Replication traffic has a separate bearer-token boundary and deliberately
	// bypasses tenant API authentication. This allows edge nodes to replicate
	// even when public ingestion uses a different authentication scheme.
	replicaHandler := replication.NewHandler(replicaStore, localNode, strings.TrimSpace(os.Getenv("TELEMETRYFORGE_EDGE_REPLICATION_TOKEN")))
	meshHandler := mesh.NewHandler(meshLocal, strings.TrimSpace(os.Getenv("TELEMETRYFORGE_MESH_TOKEN")), meshState, kafkaPublisher)
	root := http.NewServeMux()
	root.Handle("/internal/v1/mesh/", metrics.Middleware(meshHandler))
	root.Handle("/internal/v1/", metrics.Middleware(replicaHandler))
	root.Handle("/", authenticator.Middleware(metrics.Middleware(publicMux)))

	address := envOrDefault("TELEMETRYFORGE_EDGE_ADDRESS", ":8083")
	httpServer := &http.Server{Addr: address, Handler: root, ReadTimeout: cfg.ReadTimeout, WriteTimeout: maxDuration(cfg.WriteTimeout, 10*time.Second)}
	errorChannel := make(chan error, 1)
	go func() {
		stats := durablePublisher.Stats()
		logger.Info("TelemetryForge global mesh edge starting", "version", version, "edge_id", edgeID, "address", address, "wal_directory", stats.Directory, "wal_max_bytes", stats.MaxBytes, "pending_records", stats.PendingRecords, "durability_mode", stats.Replication.Mode, "replication_quorum", stats.Replication.Quorum, "configured_replication_peers", stats.Replication.ConfiguredPeers, "mesh_policy", meshManager.Config().Policy, "configured_mesh_peers", len(meshManager.Config().Peers))
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

func boolEnv(name string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
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

func maxDuration(left, right time.Duration) time.Duration {
	if left > right {
		return left
	}
	return right
}
