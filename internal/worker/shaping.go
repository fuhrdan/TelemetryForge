package worker

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/shaping"
)

// ShapingRecorder persists aggregate evidence explaining active sampling and
// candidate shadow differences.
type ShapingRecorder interface {
	ResolveShapingDecision(context.Context, shaping.Decision) (shaping.Decision, bool, error)
	RecordShapingShadowDiff(context.Context, shaping.ShadowDiff) error
}

// QueuePressure exposes current bounded worker queue utilization.
type QueuePressure interface{ Current() float64 }

// ShapingObserver receives bounded config-derived sampling outcomes.
type ShapingObserver interface {
	ShapingDecision(rule, outcome string, pressure float64)
}

// AdaptiveShaper applies the active shaping result. If audit persistence fails,
// it deliberately fails open and returns the original event unchanged rather
// than performing unaudited telemetry loss.
type AdaptiveShaper struct {
	engine    *shaping.Engine
	recorder  ShapingRecorder
	pressure  QueuePressure
	logger    *slog.Logger
	observers []ShapingObserver
}

func NewAdaptiveShaper(engine *shaping.Engine, recorder ShapingRecorder, pressure QueuePressure, logger *slog.Logger, observers ...ShapingObserver) (*AdaptiveShaper, error) {
	if engine == nil {
		return nil, errors.New("shaping engine is required")
	}
	if recorder == nil {
		return nil, errors.New("shaping recorder is required")
	}
	if pressure == nil {
		return nil, errors.New("queue pressure provider is required")
	}
	return &AdaptiveShaper{engine: engine, recorder: recorder, pressure: pressure, logger: logger, observers: observers}, nil
}

func (processor *AdaptiveShaper) Process(ctx context.Context, event domain.Event) (domain.Event, error) {
	shaped, proposed, diff := processor.engine.Evaluate(
		event, processor.pressure.Current(), time.Now().UTC(),
	)

	decision, reused, err := processor.recorder.ResolveShapingDecision(ctx, proposed)
	if err != nil {
		if processor.logger != nil {
			processor.logger.Error(
				"shaping audit persistence failed; keeping original event",
				"event_id", event.ID, "error", err,
			)
		}
		return event, nil
	}

	if reused {
		// A later processor can fail after shaping, causing the worker pool to
		// retry the whole chain. Re-evaluate at the pressure/time captured by the
		// durable first decision so the retry cannot change keep/drop behavior.
		var reconstructed shaping.Decision
		shaped, reconstructed, diff = processor.engine.Evaluate(
			event, decision.QueuePressure, decision.ObservedAt,
		)
		if !sameShapingDecision(decision, reconstructed) {
			// Reusing a name/version for different shaping content would make an
			// old decision impossible to reproduce. Preserve telemetry instead of
			// performing an ambiguous destructive action.
			if processor.logger != nil {
				processor.logger.Error(
					"stored shaping decision no longer matches config; keeping original event",
					"event_id", event.ID,
					"config", decision.ConfigName+"@"+decision.ConfigVersion,
				)
			}
			return event, nil
		}
	}

	if diff != nil {
		if err := processor.recorder.RecordShapingShadowDiff(ctx, *diff); err != nil && processor.logger != nil {
			processor.logger.Warn("shaping shadow difference persistence failed", "event_id", event.ID, "error", err)
		}
	}

	outcome := "kept"
	if decision.Protected {
		outcome = "protected"
	} else if !decision.Keep {
		outcome = "sampled_out"
	}
	rule := "default"
	if len(decision.Rules) > 0 {
		rule = decision.Rules[len(decision.Rules)-1]
	}
	for _, observer := range processor.observers {
		if observer != nil {
			observer.ShapingDecision(rule, outcome, decision.QueuePressure)
		}
	}
	if !decision.Keep {
		return shaped, stopProcessing{reason: "sampled_out"}
	}
	return shaped, nil
}

func sameShapingDecision(left, right shaping.Decision) bool {
	if left.TenantID != right.TenantID || left.EventID != right.EventID ||
		left.Source != right.Source || left.EventType != right.EventType ||
		left.ConfigName != right.ConfigName || left.ConfigVersion != right.ConfigVersion ||
		left.Keep != right.Keep || left.Protected != right.Protected ||
		left.ProtectionReason != right.ProtectionReason ||
		left.BaseRate != right.BaseRate || left.EffectiveRate != right.EffectiveRate ||
		left.QueuePressure != right.QueuePressure ||
		left.PayloadDropped != right.PayloadDropped ||
		left.OriginalBytes != right.OriginalBytes || left.ShapedBytes != right.ShapedBytes ||
		left.Reason != right.Reason || !left.ObservedAt.Equal(right.ObservedAt) {
		return false
	}
	if !sameStrings(left.Rules, right.Rules) ||
		!sameStrings(left.DroppedTags, right.DroppedTags) ||
		len(left.RenamedTags) != len(right.RenamedTags) {
		return false
	}
	for key, value := range left.RenamedTags {
		if right.RenamedTags[key] != value {
			return false
		}
	}
	return true
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

// stopProcessing is an internal successful control-flow marker. It is not an
// error condition and must never enter retry/DLQ behavior.
type stopProcessing struct{ reason string }

func (stopProcessing) Error() string  { return "stop processing" }
func isStopProcessing(err error) bool { var marker stopProcessing; return errors.As(err, &marker) }
