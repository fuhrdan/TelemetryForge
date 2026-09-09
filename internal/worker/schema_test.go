package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/reliability"
	"github.com/fuhrdan/TelemetryForge/internal/schema"
)

type memorySchemaRegistry struct {
	descriptions []schema.Description
	err          error
}

func (registry *memorySchemaRegistry) ObserveSchema(
	_ context.Context,
	_ domain.Event,
	description schema.Description,
) (schema.RegistryEntry, []schema.Drift, error) {
	if registry.err != nil {
		return schema.RegistryEntry{}, nil, registry.err
	}
	registry.descriptions = append(registry.descriptions, description)
	return schema.RegistryEntry{}, nil, nil
}

func TestSchemaInspectorRecordsNormalizedPrePolicyShape(t *testing.T) {
	registry := &memorySchemaRegistry{}
	inspector := NewSchemaInspector(registry)
	event := domain.Event{
		TenantID: "alpha", Source: " checkout ", Type: " request ",
		SchemaVersion: "1", Timestamp: time.Now().UTC(),
		Tags: map[string]string{"request_id": "abc"},
	}

	normalized, err := (Normalizer{}).Process(context.Background(), event)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := inspector.Process(context.Background(), normalized); err != nil {
		t.Fatal(err)
	}
	if len(registry.descriptions) != 1 {
		t.Fatalf("observations=%d, want 1", len(registry.descriptions))
	}
	got := registry.descriptions[0]
	if got.Source != "checkout" || got.EventType != "request" {
		t.Fatalf("schema identity source=%q type=%q", got.Source, got.EventType)
	}
	if len(got.Fields) != 1 || got.Fields[0].Path != "tags.request_id" {
		t.Fatalf("fields=%#v", got.Fields)
	}
}

func TestSchemaInspectorFailsOpenByDefault(t *testing.T) {
	registry := &memorySchemaRegistry{err: errors.New("database down")}
	inspector := NewSchemaInspector(registry)
	event := domain.Event{
		Source: "checkout", Type: "request", SchemaVersion: "1", Timestamp: time.Now().UTC(),
	}
	processed, err := inspector.Process(context.Background(), event)
	if err != nil || processed.Source != event.Source {
		t.Fatalf("fail-open result event=%#v err=%v", processed, err)
	}
}

func TestSchemaInspectorCanFailClosed(t *testing.T) {
	registry := &memorySchemaRegistry{err: errors.New("database down")}
	inspector := NewSchemaInspectorWithMode(registry, nil, false)
	_, err := inspector.Process(context.Background(), domain.Event{
		Source: "checkout", Type: "request", SchemaVersion: "1", Timestamp: time.Now().UTC(),
	})
	if err == nil || !reliability.IsTransient(err) {
		t.Fatalf("err=%v, expected transient", err)
	}
}
