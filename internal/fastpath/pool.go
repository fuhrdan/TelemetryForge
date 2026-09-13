// Package fastpath contains bounded, reusable primitives for TelemetryForge's
// performance-sensitive replay path. None of these primitives are durability
// boundaries; the WAL remains authoritative.
package fastpath

import (
	"bytes"
	"sync/atomic"
)

const defaultPoolSlots = 64

// PoolStats exposes reuse behavior without placing event- or tenant-derived
// values in metric labels.
type PoolStats struct {
	Gets        uint64 `json:"gets"`
	Reuses      uint64 `json:"reuses"`
	Allocations uint64 `json:"allocations"`
	Discards    uint64 `json:"discards"`
}

// BytePool reuses bounded byte slices through a lock-free ring. Buffers larger
// than maxCapacity are not retained, and a saturated pool discards returns
// rather than blocking a telemetry worker.
type BytePool struct {
	buffers     *Ring[[]byte]
	maxCapacity int
	gets        atomic.Uint64
	reuses      atomic.Uint64
	allocations atomic.Uint64
	discards    atomic.Uint64
}

func NewBytePool(maxCapacity int) *BytePool {
	if maxCapacity <= 0 {
		maxCapacity = 1 << 20
	}
	return &BytePool{buffers: NewRing[[]byte](defaultPoolSlots), maxCapacity: maxCapacity}
}

func (pool *BytePool) Get(size int) []byte {
	if size < 0 {
		size = 0
	}
	pool.gets.Add(1)
	for {
		buffer, ok := pool.buffers.TryPop()
		if !ok {
			break
		}
		if cap(buffer) >= size {
			pool.reuses.Add(1)
			return buffer[:size]
		}
		pool.discards.Add(1)
	}
	pool.allocations.Add(1)
	return make([]byte, size)
}

func (pool *BytePool) Put(buffer []byte) {
	if buffer == nil {
		return
	}
	if cap(buffer) > pool.maxCapacity || !pool.buffers.TryPush(buffer[:0]) {
		pool.discards.Add(1)
	}
}

func (pool *BytePool) Stats() PoolStats {
	return PoolStats{Gets: pool.gets.Load(), Reuses: pool.reuses.Load(), Allocations: pool.allocations.Load(), Discards: pool.discards.Load()}
}

// BufferPool reuses bytes.Buffer instances used by JSON encoders. The caller
// must not retain buffer.Bytes() after Put.
type BufferPool struct {
	buffers     *Ring[*bytes.Buffer]
	maxCapacity int
	gets        atomic.Uint64
	reuses      atomic.Uint64
	allocations atomic.Uint64
	discards    atomic.Uint64
}

func NewBufferPool(maxCapacity int) *BufferPool {
	if maxCapacity <= 0 {
		maxCapacity = 1 << 20
	}
	return &BufferPool{buffers: NewRing[*bytes.Buffer](defaultPoolSlots), maxCapacity: maxCapacity}
}

func (pool *BufferPool) Get() *bytes.Buffer {
	pool.gets.Add(1)
	if buffer, ok := pool.buffers.TryPop(); ok {
		pool.reuses.Add(1)
		buffer.Reset()
		return buffer
	}
	pool.allocations.Add(1)
	return &bytes.Buffer{}
}

func (pool *BufferPool) Put(buffer *bytes.Buffer) {
	if buffer == nil {
		return
	}
	if buffer.Cap() > pool.maxCapacity {
		pool.discards.Add(1)
		return
	}
	buffer.Reset()
	if !pool.buffers.TryPush(buffer) {
		pool.discards.Add(1)
	}
}

func (pool *BufferPool) Stats() PoolStats {
	return PoolStats{Gets: pool.gets.Load(), Reuses: pool.reuses.Load(), Allocations: pool.allocations.Load(), Discards: pool.discards.Load()}
}
