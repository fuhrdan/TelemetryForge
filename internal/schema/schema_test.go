package schema

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
)

func TestDescribeIsDeterministicAndDoesNotHashValues(t *testing.T) {
	first := domain.Event{
		TenantID: "alpha", Source: "checkout", Type: "request", SchemaVersion: "1",
		Timestamp: time.Now().UTC(), Tags: map[string]string{"region": "west", "service.name": "checkout"},
		Payload: json.RawMessage(`{"customer":{"id":"abc"},"ok":true}`),
	}
	second := first
	second.Tags = map[string]string{"service.name": "different-value", "region": "east"}
	second.Payload = json.RawMessage(`{"customer":{"id":"xyz"},"ok":false}`)

	one := Describe(first)
	two := Describe(second)
	if one.Fingerprint != two.Fingerprint {
		t.Fatalf("fingerprints differ for same shape: %s vs %s", one.Fingerprint, two.Fingerprint)
	}
	if one.TenantID != "alpha" || one.Source != "checkout" {
		t.Fatalf("unexpected description identity: %#v", one)
	}
}

func TestDescribeFlagsLegacySemanticConvention(t *testing.T) {
	event := domain.Event{
		Source: "api", Type: "http", SchemaVersion: "1", Timestamp: time.Now().UTC(),
		Tags: map[string]string{"http.method": "GET"},
	}
	description := Describe(event)
	if len(description.Semantic) != 1 {
		t.Fatalf("semantic findings=%d, want 1", len(description.Semantic))
	}
	if description.Semantic[0].Replacement != "http.request.method" {
		t.Fatalf("replacement=%q", description.Semantic[0].Replacement)
	}
}

func TestDiffMarksRequiredRemovalAndTypeChangeBreaking(t *testing.T) {
	from := RegistryEntry{
		Source: "checkout", EventType: "request", DeclaredVersion: "1",
		Fields: []FieldState{
			{Field: Field{Path: "payload.total", Type: TypeNumber}, Required: true},
			{Field: Field{Path: "payload.id", Type: TypeString}, Required: true},
		},
	}
	to := RegistryEntry{
		Source: "checkout", EventType: "request", DeclaredVersion: "2",
		Fields: []FieldState{
			{Field: Field{Path: "payload.total", Type: TypeString}, Required: true},
			{Field: Field{Path: "payload.currency", Type: TypeString}},
		},
	}

	diff := Diff(from, to)
	if diff.Compatibility != "breaking" {
		t.Fatalf("compatibility=%q", diff.Compatibility)
	}
	if len(diff.Changes) != 3 {
		t.Fatalf("changes=%d, want 3", len(diff.Changes))
	}
}

func TestPayloadSemanticTypeMismatch(t *testing.T) {
	event := domain.Event{
		Source: "api", Type: "http", SchemaVersion: "1", Timestamp: time.Now().UTC(),
		Payload: json.RawMessage(`{"http":{"response":{"status_code":"500"}}}`),
	}
	description := Describe(event)
	found := false
	for _, finding := range description.Semantic {
		if finding.Kind == "semantic_type_mismatch" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected semantic type mismatch")
	}
}
