package fastpath

import (
	"runtime"
	"sync/atomic"
)

// Ring is a bounded lock-free MPMC queue based on per-slot sequence numbers.
// Capacity is rounded up to a power of two. TryPush/TryPop never block; callers
// must treat a full ring as backpressure rather than silently overwriting data.
type Ring[T any] struct {
	mask    uint64
	slots   []ringSlot[T]
	enqueue atomic.Uint64
	dequeue atomic.Uint64
}

type ringSlot[T any] struct {
	sequence atomic.Uint64
	value    T
}

func NewRing[T any](capacity int) *Ring[T] {
	if capacity < 2 {
		capacity = 2
	}
	rounded := 1
	for rounded < capacity {
		rounded <<= 1
	}
	ring := &Ring[T]{mask: uint64(rounded - 1), slots: make([]ringSlot[T], rounded)}
	for index := range ring.slots {
		ring.slots[index].sequence.Store(uint64(index))
	}
	return ring
}

func (ring *Ring[T]) Capacity() int { return len(ring.slots) }

func (ring *Ring[T]) Len() int {
	enqueued := ring.enqueue.Load()
	dequeued := ring.dequeue.Load()
	if enqueued <= dequeued {
		return 0
	}
	length := enqueued - dequeued
	if length > uint64(len(ring.slots)) {
		length = uint64(len(ring.slots))
	}
	return int(length)
}

func (ring *Ring[T]) TryPush(value T) bool {
	for attempts := 0; attempts < 32; attempts++ {
		position := ring.enqueue.Load()
		slot := &ring.slots[position&ring.mask]
		sequence := slot.sequence.Load()
		difference := int64(sequence) - int64(position)
		switch {
		case difference == 0:
			if ring.enqueue.CompareAndSwap(position, position+1) {
				slot.value = value
				slot.sequence.Store(position + 1)
				return true
			}
		case difference < 0:
			return false
		default:
			runtime.Gosched()
		}
	}
	return false
}

func (ring *Ring[T]) TryPop() (T, bool) {
	var zero T
	for attempts := 0; attempts < 32; attempts++ {
		position := ring.dequeue.Load()
		slot := &ring.slots[position&ring.mask]
		sequence := slot.sequence.Load()
		difference := int64(sequence) - int64(position+1)
		switch {
		case difference == 0:
			if ring.dequeue.CompareAndSwap(position, position+1) {
				value := slot.value
				slot.value = zero
				slot.sequence.Store(position + uint64(len(ring.slots)))
				return value, true
			}
		case difference < 0:
			return zero, false
		default:
			runtime.Gosched()
		}
	}
	return zero, false
}
