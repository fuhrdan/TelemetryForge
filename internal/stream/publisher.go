// Package stream owns TelemetryForge's durable event-streaming boundary.
//
// HTTP handlers depend on the Publisher interface rather than Kafka directly.
// This keeps transport concerns separate from broker concerns and lets later
// replay, shadow-pipeline, and test tooling reuse the same publishing contract.
package stream

import (
	"context"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
)

// Publisher durably hands accepted telemetry to the streaming layer.
//
// Publish returns only after the configured broker acknowledgement policy has
// succeeded or the request context has expired. Ready reports whether the
// publisher can currently reach its required downstream dependency.
type Publisher interface {
	Publish(ctx context.Context, topic string, event domain.Event) error
	Ready(ctx context.Context) error
	Close()
}
