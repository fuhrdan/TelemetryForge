// Package config contains runtime configuration for TelemetryForge services.
package config

import (
	"os"
	"strings"
	"time"
)

// Config contains gateway runtime settings.
type Config struct {
	Address          string
	ReadTimeout      time.Duration
	WriteTimeout     time.Duration
	ShutdownTimeout  time.Duration
	KafkaBrokers     []string
	KafkaClientID    string
	KafkaRawTopic    string
	KafkaMetricTopic string
	KafkaTimeout     time.Duration
	DatabaseURL      string
}

// Load reads gateway configuration from environment variables and applies
// conservative local-development defaults where values are not provided.
func Load() Config {
	return Config{
		Address:          envOrDefault("TELEMETRYFORGE_ADDRESS", ":8080"),
		ReadTimeout:      5 * time.Second,
		WriteTimeout:     10 * time.Second,
		ShutdownTimeout:  10 * time.Second,
		KafkaBrokers:     splitCSV(envOrDefault("TELEMETRYFORGE_KAFKA_BROKERS", "localhost:9092")),
		KafkaClientID:    envOrDefault("TELEMETRYFORGE_KAFKA_CLIENT_ID", "telemetryforge-gateway"),
		KafkaRawTopic:    envOrDefault("TELEMETRYFORGE_KAFKA_RAW_TOPIC", "telemetry.raw"),
		KafkaMetricTopic: envOrDefault("TELEMETRYFORGE_KAFKA_METRIC_TOPIC", "telemetry.metrics"),
		KafkaTimeout:     5 * time.Second,
		DatabaseURL:      envOrDefault("TELEMETRYFORGE_DATABASE_URL", "postgres://telemetryforge:telemetryforge@localhost:5432/telemetryforge?sslmode=disable"),
	}
}

func envOrDefault(name string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
