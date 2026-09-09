package storage

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/schema"
)

func TestUpdateRegistryDoesNotRequireSparseFieldBeforeSampleFloor(t *testing.T) {
	now := time.Now().UTC()
	entry := schema.RegistryEntry{
		TenantID: "alpha", Source: "orders", EventType: "created", DeclaredVersion: "1",
		Health: "healthy", FirstSeen: now.Add(-time.Minute), LastSeen: now,
		ObservationCount: 5,
		Fields: []schema.FieldState{
			{Field: schema.Field{Path: "payload.id", Type: schema.TypeString}, SeenCount: 5},
			{Field: schema.Field{Path: "payload.note", Type: schema.TypeString}, SeenCount: 2},
		},
	}
	description := schema.Description{
		TenantID: "alpha", Source: "orders", EventType: "created", DeclaredVersion: "1",
		Fields: []schema.Field{{Path: "payload.id", Type: schema.TypeString}},
	}

	updated, drifts := updateRegistryEntry(entry, description, now.Add(time.Second))
	for _, drift := range drifts {
		if drift.Kind == "required_field_missing" {
			t.Fatalf("premature required-field drift: %#v", drift)
		}
	}
	if updated.RequiredFieldCount != 0 {
		t.Fatalf("required fields=%d before sample floor", updated.RequiredFieldCount)
	}
}

func TestUpdateRegistryFlagsEstablishedTypeChangeBreaking(t *testing.T) {
	now := time.Now().UTC()
	entry := schema.RegistryEntry{
		TenantID: "alpha", Source: "orders", EventType: "created", DeclaredVersion: "1",
		Health: "healthy", FirstSeen: now.Add(-time.Hour), LastSeen: now,
		ObservationCount: 24,
		Fields: []schema.FieldState{{
			Field:     schema.Field{Path: "payload.total", Type: schema.TypeNumber},
			SeenCount: 24, Required: true,
		}},
	}
	description := schema.Description{
		TenantID: "alpha", Source: "orders", EventType: "created", DeclaredVersion: "1",
		Fields: []schema.Field{{Path: "payload.total", Type: schema.TypeString}},
	}

	updated, drifts := updateRegistryEntry(entry, description, now.Add(time.Second))
	found := false
	for _, drift := range drifts {
		if drift.Kind == "type_changed" && drift.Severity == "breaking" {
			found = true
		}
	}
	if !found {
		t.Fatalf("drifts=%#v, expected breaking type change", drifts)
	}
	if updated.Health != "breaking" {
		t.Fatalf("health=%q", updated.Health)
	}
}

func TestSchemaURLChangeWithinSameVersionWarns(t *testing.T) {
	now := time.Now().UTC()
	entry := schema.RegistryEntry{
		TenantID: "alpha", Source: "api", EventType: "http", DeclaredVersion: "1",
		SchemaURL: "https://opentelemetry.io/schemas/1.43.0",
		Health:    "healthy", FirstSeen: now, LastSeen: now, ObservationCount: 1,
	}
	description := schema.Description{
		TenantID: "alpha", Source: "api", EventType: "http", DeclaredVersion: "1",
		SchemaURL: "https://opentelemetry.io/schemas/1.44.0",
	}

	updated, drifts := updateRegistryEntry(entry, description, now.Add(time.Second))
	if updated.SchemaURL != description.SchemaURL {
		t.Fatalf("schema URL=%q", updated.SchemaURL)
	}
	found := false
	for _, drift := range drifts {
		if drift.Kind == "schema_url_changed" && drift.Severity == "warning" {
			found = true
		}
	}
	if !found {
		t.Fatalf("drifts=%#v", drifts)
	}
}

func TestUpdateRegistryCapsAccumulatedFieldPaths(t *testing.T) {
	now := time.Now().UTC()
	fields := make([]schema.FieldState, 0, schema.MaxRegistryFields)
	for index := 0; index < schema.MaxRegistryFields; index++ {
		fields = append(fields, schema.FieldState{
			Field: schema.Field{
				Path: "payload.existing_" + fmt.Sprint(index),
				Type: schema.TypeString,
			},
			SeenCount: 1,
		})
	}
	entry := schema.RegistryEntry{
		TenantID: "alpha", Source: "orders", EventType: "created", DeclaredVersion: "1",
		Health: "healthy", FirstSeen: now, LastSeen: now, ObservationCount: 1,
		Fields: fields, FieldCount: len(fields),
	}
	description := schema.Description{
		TenantID: "alpha", Source: "orders", EventType: "created", DeclaredVersion: "1",
		Fields: []schema.Field{{Path: "payload.one_more", Type: schema.TypeString}},
	}

	updated, drifts := updateRegistryEntry(entry, description, now.Add(time.Second))
	if len(updated.Fields) != schema.MaxRegistryFields {
		t.Fatalf("fields=%d, want cap %d", len(updated.Fields), schema.MaxRegistryFields)
	}
	found := false
	for _, drift := range drifts {
		if drift.Kind == "registry_field_limit" && drift.Severity == "warning" {
			found = true
		}
	}
	if !found {
		t.Fatalf("drifts=%#v, expected registry field-limit warning", drifts)
	}
}

func TestPruneSchemaObservationsRequiresCutoff(t *testing.T) {
	store := &PostgresStore{}
	if _, err := store.PruneSchemaObservations(context.Background(), time.Time{}); err == nil {
		t.Fatal("expected zero prune cutoff to fail before touching database")
	}
}
