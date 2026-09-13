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

// BatchItem is one ordered event handed to an optional batch-capable publisher.
type BatchItem struct {
	Topic string
	Event domain.Event
}

// BatchPublisher is an optional fast-path extension. Implementations return one
// error slot per input item. A nil slot means that item was acknowledged. The
// durable edge only checkpoints the contiguous successful prefix, so partial
// batch failures preserve at-least-once replay semantics.
type BatchPublisher interface {
	PublishBatch(ctx context.Context, items []BatchItem) []error
}
