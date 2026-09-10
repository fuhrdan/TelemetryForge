package shaping

import (
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
)

// PreviewEvents evaluates historical events without writing production state.
func PreviewEvents(config Config, events []domain.Event, pressure float64) (Preview, error) {
	engine, err := NewEngine(config, nil)
	if err != nil {
		return Preview{}, err
	}
	preview := Preview{ConfigName: config.Name, ConfigVersion: config.Version, QueuePressure: clamp(pressure, 0, 1)}
	observedKinds := map[string]struct{}{}
	keptKinds := map[string]struct{}{}
	for _, event := range events {
		_, decision, _ := engine.Evaluate(event, pressure, event.Timestamp)
		preview.Observed++
		preview.OriginalBytes += int64(decision.OriginalBytes)
		preview.ShapedBytes += int64(decision.ShapedBytes)
		kind := event.Source + "\x00" + event.Type
		observedKinds[kind] = struct{}{}
		if decision.Protected {
			preview.ProtectedObserved++
		}
		if decision.Keep {
			preview.Kept++
			keptKinds[kind] = struct{}{}
			if decision.Protected {
				preview.ProtectedKept++
			}
		} else {
			preview.SampledOut++
		}
		if len(decision.DroppedTags) > 0 || len(decision.RenamedTags) > 0 || decision.PayloadDropped {
			preview.Transformed++
		}
		if decision.PayloadDropped {
			preview.PayloadDropped++
		}
	}
	preview.EventRetentionPercent = percent(preview.Kept, preview.Observed)
	if preview.OriginalBytes > 0 {
		preview.ByteRetentionPercent = float64(preview.ShapedBytes) * 100 / float64(preview.OriginalBytes)
	}
	preview.ProtectedRetentionPercent = percent(preview.ProtectedKept, preview.ProtectedObserved)
	preview.SourceTypeCoveragePercent = percent(len(keptKinds), len(observedKinds))
	_ = time.UTC
	return preview, nil
}

func percent(numerator, denominator int) float64 {
	if denominator == 0 {
		return 100
	}
	return float64(numerator) * 100 / float64(denominator)
}
