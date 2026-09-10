package router

import (
	"context"
	"fmt"
	"log/slog"
	"math"

	"github.com/fuhrdan/TelemetryForge/internal/connectors"
	"sync"
	"time"
)

// DeliveryStore is the durable outbox boundary used by the router service.
type DeliveryStore interface {
	ClaimRoutingDelivery(context.Context, string, time.Duration) (Delivery, bool, error)
	MarkRoutingDelivered(context.Context, Delivery) error
	MarkRoutingRetry(context.Context, Delivery, string, time.Time) error
	DeadLetterRoutingDelivery(context.Context, Delivery, string, *Decision) error
	RecordRoutingDestinationHealth(context.Context, string, string, bool, string) error
}

// Observer receives low-cardinality route-delivery metrics.
type Observer interface {
	RoutingDelivery(destination, outcome string)
}

// Dispatcher drains destination-specific outbox lanes independently.
type Dispatcher struct {
	config       Config
	store        DeliveryStore
	senders      map[string]Sender
	logger       *slog.Logger
	observer     Observer
	pollInterval time.Duration
	lease        time.Duration
	closeOnce    sync.Once
}

// NewDispatcher builds all enabled destination clients up front so invalid
// endpoints/configuration fail before the service begins claiming work.
func NewDispatcher(
	config Config,
	store DeliveryStore,
	factory SenderFactoryConfig,
	logger *slog.Logger,
	observer Observer,
) (*Dispatcher, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if store == nil {
		return nil, fmt.Errorf("routing delivery store is required")
	}
	if logger == nil {
		logger = slog.Default()
	}

	dispatcher := &Dispatcher{
		config: config, store: store, logger: logger, observer: observer,
		senders: make(map[string]Sender), pollInterval: 250 * time.Millisecond,
		lease: 30 * time.Second,
	}
	for _, destination := range config.EnabledDestinations() {
		sender, err := NewSender(destination, factory)
		if err != nil {
			dispatcher.Close()
			return nil, fmt.Errorf("destination %q: %w", destination.Name, err)
		}
		dispatcher.senders[destination.Name] = sender
	}
	return dispatcher, nil
}

// Run starts one isolated worker lane per configured destination/concurrency.
// One unhealthy destination can accumulate retry/DLQ state without blocking
// another destination's goroutines.
func (dispatcher *Dispatcher) Run(ctx context.Context) {
	var wait sync.WaitGroup
	for _, destination := range dispatcher.config.EnabledDestinations() {
		for workerIndex := 0; workerIndex < destination.Concurrency; workerIndex++ {
			wait.Add(1)
			go func(destination Destination, lane int) {
				defer wait.Done()
				dispatcher.runLane(ctx, destination, lane)
			}(destination, workerIndex)
		}
	}
	<-ctx.Done()
	wait.Wait()
}

// ConnectorRuntimeStates probes configured destinations independently from process readiness.
func (dispatcher *Dispatcher) ConnectorRuntimeStates(ctx context.Context, instanceID string) []connectors.RuntimeState {
	destinations := dispatcher.config.EnabledDestinations()
	result := make([]connectors.RuntimeState, len(destinations))
	var wait sync.WaitGroup
	for index, destination := range destinations {
		sender, ok := dispatcher.senders[destination.Name]
		if !ok {
			continue
		}
		wait.Add(1)
		go func(index int, destination Destination, sender Sender) {
			defer wait.Done()
			state := connectors.RuntimeState{InstanceID: instanceID, Destination: destination.Name, Kind: sender.Kind(), Capabilities: sender.Capabilities(), Ready: true, CheckedAt: time.Now().UTC()}
			probeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			if err := sender.Ready(probeCtx); err != nil {
				state.Ready = false
				state.LastError = err.Error()
			}
			result[index] = state
		}(index, destination, sender)
	}
	wait.Wait()
	compacted := result[:0]
	for _, state := range result {
		if state.Destination != "" {
			compacted = append(compacted, state)
		}
	}
	return compacted
}

// Ready intentionally checks initialization rather than every backend. Making
// Kubernetes restart the entire router because one destination is down would
// defeat destination failure isolation.
func (dispatcher *Dispatcher) Ready(context.Context) error {
	if dispatcher == nil || len(dispatcher.senders) == 0 {
		return fmt.Errorf("router has no initialized destinations")
	}
	return nil
}

func (dispatcher *Dispatcher) Close() {
	dispatcher.closeOnce.Do(func() {
		for _, sender := range dispatcher.senders {
			sender.Close()
		}
	})
}

func (dispatcher *Dispatcher) runLane(ctx context.Context, destination Destination, lane int) {
	for {
		if ctx.Err() != nil {
			return
		}
		delivery, ok, err := dispatcher.store.ClaimRoutingDelivery(ctx, destination.Name, dispatcher.lease)
		if err != nil {
			dispatcher.logger.Warn("routing outbox claim failed", "destination", destination.Name, "lane", lane, "error", err)
			if !sleepContext(ctx, dispatcher.pollInterval) {
				return
			}
			continue
		}
		if !ok {
			if !sleepContext(ctx, dispatcher.pollInterval) {
				return
			}
			continue
		}
		dispatcher.deliver(ctx, delivery)
	}
}

func (dispatcher *Dispatcher) deliver(ctx context.Context, delivery Delivery) {
	sender, ok := dispatcher.senders[delivery.Destination]
	if !ok {
		dispatcher.terminalFailure(ctx, delivery, fmt.Errorf("destination is no longer configured"))
		return
	}

	err := sender.Send(ctx, delivery.Event)
	if err == nil {
		if markErr := dispatcher.store.MarkRoutingDelivered(ctx, delivery); markErr != nil {
			dispatcher.logger.Error("mark routed delivery complete", "destination", delivery.Destination, "event_id", delivery.EventID, "error", markErr)
			return
		}
		_ = dispatcher.store.RecordRoutingDestinationHealth(ctx, delivery.TenantID, delivery.Destination, true, "")
		dispatcher.observe(delivery.Destination, "delivered")
		return
	}

	_ = dispatcher.store.RecordRoutingDestinationHealth(ctx, delivery.TenantID, delivery.Destination, false, err.Error())
	if !connectors.Retryable(err) {
		dispatcher.observe(delivery.Destination, "permanent_failure")
		dispatcher.terminalFailure(ctx, delivery, err)
		return
	}
	if delivery.Attempts >= delivery.MaxAttempts {
		dispatcher.terminalFailure(ctx, delivery, err)
		return
	}

	delay := retryDelay(delivery.Attempts, delivery.BaseDelayMS, delivery.MaxDelayMS)
	if markErr := dispatcher.store.MarkRoutingRetry(ctx, delivery, err.Error(), time.Now().UTC().Add(delay)); markErr != nil {
		dispatcher.logger.Error("schedule routed delivery retry", "destination", delivery.Destination, "event_id", delivery.EventID, "error", markErr)
		return
	}
	dispatcher.observe(delivery.Destination, "retry")
}

func (dispatcher *Dispatcher) terminalFailure(ctx context.Context, delivery Delivery, cause error) {
	var fallback *Decision
	if delivery.FailureFallback != "" {
		if destination, ok := dispatcher.config.DestinationByName(delivery.FailureFallback); ok && destination.Enabled {
			fallback = &Decision{
				Destination: destination.Name, RouteRules: []string{"failure_fallback:" + delivery.Destination},
				MaxAttempts: destination.MaxAttempts, BaseDelayMS: destination.BaseDelayMS,
				MaxDelayMS: destination.MaxDelayMS, FailureFallback: destination.FailureFallback,
			}
		}
	}
	if err := dispatcher.store.DeadLetterRoutingDelivery(ctx, delivery, cause.Error(), fallback); err != nil {
		dispatcher.logger.Error("dead-letter routed delivery", "destination", delivery.Destination, "event_id", delivery.EventID, "error", err)
		return
	}
	dispatcher.observe(delivery.Destination, "dead_letter")
	if fallback != nil {
		dispatcher.observe(fallback.Destination, "fallback_enqueued")
	}
}

func (dispatcher *Dispatcher) observe(destination, outcome string) {
	if dispatcher.observer != nil {
		dispatcher.observer.RoutingDelivery(destination, outcome)
	}
}

func retryDelay(attempt, baseMS, maxMS int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if baseMS < 1 {
		baseMS = 500
	}
	if maxMS < baseMS {
		maxMS = baseMS
	}
	power := math.Pow(2, float64(attempt-1))
	delay := time.Duration(float64(baseMS)*power) * time.Millisecond
	maximum := time.Duration(maxMS) * time.Millisecond
	if delay > maximum {
		return maximum
	}
	return delay
}

func sleepContext(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
