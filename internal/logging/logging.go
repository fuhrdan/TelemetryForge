// Package logging centralizes structured logging configuration.
package logging

import (
	"log/slog"
	"os"
)

// New creates the default JSON logger used by TelemetryForge services.
// Structured JSON output makes local logs easy to inspect while remaining
// compatible with container and centralized logging systems.
func New() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, nil))
}
