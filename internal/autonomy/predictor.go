package autonomy

import (
	"math"
	"time"
)

// ForecastPressure performs a bounded least-squares trend projection. It is a
// transparent predictor, not an opaque ML/root-cause model.
func ForecastPressure(history []Observation, horizon time.Duration, now time.Time) Forecast {
	result := Forecast{GeneratedAt: now.UTC(), Horizon: horizon, Samples: len(history), Confidence: "insufficient"}
	if len(history) == 0 {
		return result
	}
	result.CurrentPressure = clamp(history[len(history)-1].QueuePressure, 0, 1)
	result.PredictedPressure = result.CurrentPressure
	if len(history) < 2 {
		return result
	}

	origin := history[0].At
	var sumX, sumY, sumXY, sumXX float64
	for _, sample := range history {
		x := sample.At.Sub(origin).Seconds()
		y := clamp(sample.QueuePressure, 0, 1)
		sumX += x
		sumY += y
		sumXY += x * y
		sumXX += x * x
	}
	n := float64(len(history))
	denom := n*sumXX - sumX*sumX
	if math.Abs(denom) < 1e-12 {
		return result
	}
	slope := (n*sumXY - sumX*sumY) / denom
	result.SlopePerSecond = slope
	projected := result.CurrentPressure + slope*horizon.Seconds()
	result.PredictedPressure = clamp(projected, 0, 1)
	switch {
	case len(history) >= 8:
		result.Confidence = "high"
	case len(history) >= 4:
		result.Confidence = "medium"
	default:
		result.Confidence = "low"
	}
	return result
}

func clamp(value, low, high float64) float64 {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}
