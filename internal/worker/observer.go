package worker

import "time"

// CompositeObserver fans bounded-worker observations to multiple independent
// consumers such as Prometheus metrics and the adaptive-sampling pressure
// controller.
type CompositeObserver struct{ observers []Observer }

func NewCompositeObserver(observers ...Observer) *CompositeObserver {
	filtered := make([]Observer, 0, len(observers))
	for _, observer := range observers {
		if observer != nil {
			filtered = append(filtered, observer)
		}
	}
	return &CompositeObserver{observers: filtered}
}
func (observer *CompositeObserver) QueueDepth(current, capacity int) {
	for _, item := range observer.observers {
		item.QueueDepth(current, capacity)
	}
}
func (observer *CompositeObserver) JobCompleted(duration time.Duration, attempts int, outcome string) {
	for _, item := range observer.observers {
		item.JobCompleted(duration, attempts, outcome)
	}
}
func (observer *CompositeObserver) Retry(classification string) {
	for _, item := range observer.observers {
		item.Retry(classification)
	}
}
