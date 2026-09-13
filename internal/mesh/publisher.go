package mesh

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/stream"
)

// Downstream is the minimal durable publishing contract required by mesh.
type Downstream interface {
	Publish(context.Context, string, domain.Event) error
	Ready(context.Context) error
	Close()
}

// Publisher routes the origin WAL replay to the deterministic healthy mesh
// owner. Local delivery uses the existing Kafka publisher; remote forwarding is
// terminal at the receiving node to prevent routing loops.
type Publisher struct {
	manager *Manager
	local   Downstream
	client  *http.Client
	token   string
}

func NewPublisher(manager *Manager, local Downstream) *Publisher {
	return &Publisher{manager: manager, local: local, client: &http.Client{Timeout: manager.config.Timeout}, token: manager.config.Token}
}

// PublishBatch keeps the common local-owner path batched. If any item maps to
// a remote owner, or if the local batch fails, it falls back to normal per-item
// mesh delivery so each record retains existing failover behavior.
func (publisher *Publisher) PublishBatch(ctx context.Context, items []stream.BatchItem) []error {
	results := make([]error, len(items))
	if len(items) == 0 {
		return results
	}
	batchLocal, ok := publisher.local.(stream.BatchPublisher)
	if ok {
		allLocal := true
		for index, item := range items {
			route, err := publisher.manager.Route(ctx, routeKey(item.Event))
			if err != nil {
				results[index] = err
				allLocal = false
				continue
			}
			if route.Selected.ID != publisher.manager.config.Local.ID {
				allLocal = false
			}
		}
		if allLocal {
			return batchLocal.PublishBatch(ctx, items)
		}
	}
	for index, item := range items {
		if results[index] != nil {
			continue
		}
		results[index] = publisher.Publish(ctx, item.Topic, item.Event)
	}
	return results
}

func (publisher *Publisher) Publish(ctx context.Context, topic string, event domain.Event) error {
	key := routeKey(event)
	route, err := publisher.manager.Route(ctx, key)
	if err != nil {
		return err
	}
	var failures []error
	for _, candidate := range route.Candidates {
		node := candidate.Node
		if node.ID == publisher.manager.config.Local.ID {
			if err := publisher.local.Publish(ctx, topic, event); err == nil {
				return nil
			} else {
				failures = append(failures, fmt.Errorf("local node %s: %w", node.ID, err))
			}
			continue
		}
		forward := ForwardRequest{TargetNodeID: node.ID, RouteKey: key, Topic: topic, Event: event}
		if err := forwardToPeer(ctx, publisher.client, node, publisher.token, forward); err == nil {
			publisher.manager.ReportSuccess(node.ID)
			return nil
		} else {
			publisher.manager.ReportFailure(node.ID, err)
			failures = append(failures, fmt.Errorf("mesh node %s: %w", node.ID, err))
		}
	}
	if len(failures) == 0 {
		return ErrNoRoute
	}
	return fmt.Errorf("%w: %v", ErrNoRoute, errors.Join(failures...))
}

func (publisher *Publisher) Ready(ctx context.Context) error {
	_, err := publisher.manager.Route(ctx, "__telemetryforge_readiness__")
	return err
}

func (publisher *Publisher) Close() { publisher.local.Close() }

func routeKey(event domain.Event) string {
	parts := []string{strings.TrimSpace(event.TenantID), strings.TrimSpace(event.Source)}
	key := strings.Join(parts, "|")
	if strings.Trim(key, "|") == "" {
		key = event.ID
	}
	return key
}
