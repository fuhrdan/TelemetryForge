package shaping

import (
	"math"
	"sync/atomic"
	"time"
)

// PressureController receives bounded worker queue observations and exposes a
// lock-free utilization ratio to the shaping processor.
type PressureController struct{ bits atomic.Uint64 }

func NewPressureController() *PressureController { return &PressureController{} }
func (controller *PressureController) QueueDepth(current, capacity int) {
	ratio := 0.0
	if capacity > 0 {
		ratio = float64(current) / float64(capacity)
	}
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	controller.bits.Store(math.Float64bits(ratio))
}
func (controller *PressureController) Current() float64 {
	return math.Float64frombits(controller.bits.Load())
}

// The remaining methods make the controller compatible with worker.Observer.
func (*PressureController) JobCompleted(time.Duration, int, string) {}
func (*PressureController) Retry(string)                            {}
