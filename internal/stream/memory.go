package stream

import (
	"context"
	"sync"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
)

// PublishedEvent records a topic/event pair captured by MemoryPublisher.
type PublishedEvent struct {
	Topic string
	Event domain.Event
}

// MemoryPublisher is a lightweight Publisher used by API tests.
// It intentionally mirrors the synchronous success behavior of the Kafka
// implementation without requiring a broker during unit tests.
type MemoryPublisher struct {
	mutex  sync.Mutex
	events []PublishedEvent
	err    error
}

// NewMemoryPublisher creates an empty in-memory publisher.
func NewMemoryPublisher() *MemoryPublisher {
	return &MemoryPublisher{}
}

// Publish records an event unless a test error has been configured.
func (publisher *MemoryPublisher) Publish(_ context.Context, topic string, event domain.Event) error {
	publisher.mutex.Lock()
	defer publisher.mutex.Unlock()

	if publisher.err != nil {
		return publisher.err
	}

	publisher.events = append(publisher.events, PublishedEvent{Topic: topic, Event: event})
	return nil
}

// PublishBatch records an ordered batch and returns one result per item.
func (publisher *MemoryPublisher) PublishBatch(_ context.Context, items []BatchItem) []error {
	publisher.mutex.Lock()
	defer publisher.mutex.Unlock()
	results := make([]error, len(items))
	if publisher.err != nil {
		for index := range results {
			results[index] = publisher.err
		}
		return results
	}
	for _, item := range items {
		publisher.events = append(publisher.events, PublishedEvent{Topic: item.Topic, Event: item.Event})
	}
	return results
}

// Ready returns the configured test error, if any.
func (publisher *MemoryPublisher) Ready(_ context.Context) error {
	publisher.mutex.Lock()
	defer publisher.mutex.Unlock()
	return publisher.err
}

// Close satisfies Publisher; no resources are held by MemoryPublisher.
func (publisher *MemoryPublisher) Close() {}

// Events returns a defensive copy of captured publications.
func (publisher *MemoryPublisher) Events() []PublishedEvent {
	publisher.mutex.Lock()
	defer publisher.mutex.Unlock()

	result := make([]PublishedEvent, len(publisher.events))
	copy(result, publisher.events)
	return result
}

// SetError configures publication and readiness failures for tests.
func (publisher *MemoryPublisher) SetError(err error) {
	publisher.mutex.Lock()
	defer publisher.mutex.Unlock()
	publisher.err = err
}
