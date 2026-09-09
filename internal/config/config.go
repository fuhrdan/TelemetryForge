// Package config contains runtime configuration for TelemetryForge services.
package config

import (
	"os"
	"time"
)

// Config contains gateway runtime settings.
type Config struct {
	Address         string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
}

// Load reads gateway configuration from environment variables and applies
// conservative local-development defaults where values are not provided.
func Load() Config {
	address := os.Getenv("TELEMETRYFORGE_ADDRESS")
	if address == "" {
		address = ":8080"
	}

	return Config{
		Address:         address,
		ReadTimeout:     5 * time.Second,
		WriteTimeout:    10 * time.Second,
		ShutdownTimeout: 10 * time.Second,
	}
}
