package security

import (
	"encoding/json"
	"testing"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
)

func TestRedactorHidesConfiguredTagsAndPayload(t *testing.T) {
	redactor := NewRedactor("email,authorization", true)
	event := domain.Event{
		Tags: map[string]string{
			"email":  "person@example.com",
			"region": "us-west",
		},
		Payload: json.RawMessage(`{"card":"4111111111111111"}`),
	}

	result := redactor.Event(event)
	if result.Tags["email"] != redactedValue {
		t.Fatalf("email=%q", result.Tags["email"])
	}
	if result.Tags["region"] != "us-west" {
		t.Fatalf("region=%q", result.Tags["region"])
	}
	if string(result.Payload) != `{"redacted":true}` {
		t.Fatalf("payload=%s", result.Payload)
	}

	// Stored/source event remains unchanged.
	if event.Tags["email"] != "person@example.com" {
		t.Fatal("redaction mutated the source event")
	}
}
