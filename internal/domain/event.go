package domain

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// Event represents the canonical telemetry envelope accepted by TelemetryForge.
//
// The envelope is intentionally generic so logs, metrics, traces, webhook
// payloads, and custom application events can share the same ingestion path.
// Payload remains schema-flexible while the top-level routing and correlation
// fields stay stable across event types.
type Event struct {
	ID            string            `json:"id,omitempty"`
	TenantID      string            `json:"tenant_id,omitempty"`
	Source        string            `json:"source"`
	Type          string            `json:"type"`
	Timestamp     time.Time         `json:"timestamp"`
	Tags          map[string]string `json:"tags,omitempty"`
	Payload       json.RawMessage   `json:"payload,omitempty"`
	Value         *float64          `json:"value,omitempty"`
	Unit          string            `json:"unit,omitempty"`
	SchemaVersion string            `json:"schema_version"`
	SchemaURL     string            `json:"schema_url,omitempty"`
	CorrelationID string            `json:"correlation_id,omitempty"`
}

// Validate checks the minimum invariants required before an event is accepted
// into the TelemetryForge pipeline.
//
// Validation is deliberately kept separate from HTTP handling so the same
// rules can later be reused by gRPC ingestion, Kafka replay, and CLI tooling.
func (event Event) Validate() error {
	if strings.TrimSpace(event.Source) == "" {
		return errors.New("source is required")
	}

	if strings.TrimSpace(event.Type) == "" {
		return errors.New("type is required")
	}

	if event.Timestamp.IsZero() {
		return errors.New("timestamp is required")
	}

	if strings.TrimSpace(event.SchemaVersion) == "" {
		return errors.New("schema_version is required")
	}

	return nil
}
