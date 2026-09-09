// Package worker implements the bounded stream-processing stage.
//
// The worker package intentionally owns no HTTP or Kafka-specific behavior.
// A Consumer feeds canonical domain events into a Pool, and a Processor
// performs normalization or enrichment. Keeping these responsibilities
// separate makes processing rules straightforward to unit test and replay.
package worker

import (
	"context"
	"errors"
	"strings"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/reliability"
)

// Processor transforms or enriches one canonical telemetry event.
type Processor interface {
	Process(ctx context.Context, event domain.Event) (domain.Event, error)
}

// Normalizer performs the first intentionally small processing step.
//
// Normalizer trims source and type whitespace as the first canonicalization
// step. It remains deliberately small so later processors stay composable.
type Normalizer struct{}

// Process trims fields whose accidental whitespace would otherwise create
// distinct sources or event types downstream.
func (Normalizer) Process(_ context.Context, event domain.Event) (domain.Event, error) {
	event.Source = strings.TrimSpace(event.Source)
	event.Type = strings.TrimSpace(event.Type)
	if event.Source == "" || event.Type == "" {
		return domain.Event{}, reliability.New(reliability.Permanent, "normalize", errors.New("normalized event has empty source or type"))
	}
	return event, nil
}
