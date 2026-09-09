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
	"github.com/fuhrdan/TelemetryForge/internal/stream"
)

func main() {
	logger := logging.New()
	cfg := config.Load()

	publisher, err := stream.NewKafkaPublisher(stream.KafkaConfig{
		Brokers:        cfg.KafkaBrokers,
		ClientID:       cfg.KafkaClientID,
		ProduceTimeout: cfg.KafkaTimeout,
	}, logger)
	if err != nil {
		logger.Error("Kafka publisher initialization failed", "error", err)
		os.Exit(1)
	}
	defer publisher.Close()

	handler := api.NewServer(logger, publisher, api.Topics{
		Raw:    cfg.KafkaRawTopic,
		Metric: cfg.KafkaMetricTopic,
	})

	httpServer := &http.Server{
		Addr:         cfg.Address,
		Handler:      handler,
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
