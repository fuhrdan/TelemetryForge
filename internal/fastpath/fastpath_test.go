package fastpath

import (
	"encoding/json"
	"sync"
	"testing"
)

func TestRingFIFOAndBounds(t *testing.T) {
	ring := NewRing[int](3)
	if ring.Capacity() != 4 {
		t.Fatalf("capacity=%d want 4", ring.Capacity())
	}
	for index := 1; index <= 4; index++ {
		if !ring.TryPush(index) {
			t.Fatalf("push %d failed", index)
		}
	}
	if ring.TryPush(5) {
		t.Fatal("full ring accepted overwrite")
	}
	for index := 1; index <= 4; index++ {
		value, ok := ring.TryPop()
		if !ok || value != index {
			t.Fatalf("pop=(%d,%v) want (%d,true)", value, ok, index)
		}
	}
	if _, ok := ring.TryPop(); ok {
		t.Fatal("empty ring returned a value")
	}
}

func TestRingConcurrentProducersConsumers(t *testing.T) {
	const producers = 4
	const perProducer = 1000
	ring := NewRing[int](256)
	var producersWG sync.WaitGroup
	for producer := 0; producer < producers; producer++ {
		producersWG.Add(1)
		go func(base int) {
			defer producersWG.Done()
			for index := 0; index < perProducer; {
				if ring.TryPush(base*perProducer + index) {
					index++
				}
			}
		}(producer)
	}
	seen := make(map[int]struct{}, producers*perProducer)
	for len(seen) < producers*perProducer {
		if value, ok := ring.TryPop(); ok {
			if _, duplicate := seen[value]; duplicate {
				t.Fatalf("duplicate value %d", value)
			}
			seen[value] = struct{}{}
		}
	}
	producersWG.Wait()
}

func TestPoolsReuseAndBoundRetention(t *testing.T) {
	bytesPool := NewBytePool(128)
	first := bytesPool.Get(64)
	bytesPool.Put(first)
	second := bytesPool.Get(32)
	bytesPool.Put(second)
	bytesPool.Put(make([]byte, 0, 1024))
	stats := bytesPool.Stats()
	if stats.Gets != 2 || stats.Allocations == 0 || stats.Discards == 0 {
		t.Fatalf("unexpected stats: %+v", stats)
	}

	bufferPool := NewBufferPool(128)
	buffer := bufferPool.Get()
	buffer.WriteString("hello")
	bufferPool.Put(buffer)
	buffer = bufferPool.Get()
	if buffer.Len() != 0 {
		t.Fatalf("reused buffer length=%d", buffer.Len())
	}
	bufferPool.Put(buffer)
}

func BenchmarkRingRoundTrip(b *testing.B) {
	ring := NewRing[int](1024)
	b.ReportAllocs()
	for index := 0; index < b.N; index++ {
		if !ring.TryPush(index) {
			b.Fatal("unexpected full ring")
		}
		if _, ok := ring.TryPop(); !ok {
			b.Fatal("unexpected empty ring")
		}
	}
}

func BenchmarkBytePoolRoundTrip(b *testing.B) {
	pool := NewBytePool(64 << 10)
	b.ReportAllocs()
	for index := 0; index < b.N; index++ {
		buffer := pool.Get(4096)
		buffer[0] = byte(index)
		pool.Put(buffer)
	}
}

func BenchmarkJSONBufferReuse(b *testing.B) {
	pool := NewBufferPool(64 << 10)
	value := struct {
		ID string `json:"id"`
		N  int    `json:"n"`
	}{ID: "evt-1234567890", N: 42}
	b.ReportAllocs()
	for index := 0; index < b.N; index++ {
		buffer := pool.Get()
		if err := json.NewEncoder(buffer).Encode(value); err != nil {
			b.Fatal(err)
		}
		pool.Put(buffer)
	}
}
