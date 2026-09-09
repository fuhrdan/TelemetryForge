package domain

import (
	"testing"
	"time"
)

func TestEventValidate(t *testing.T) {
	valid := Event{
		Source:        "checkout-api",
		Type:          "request.duration",
		Timestamp:     time.Now().UTC(),
		SchemaVersion: "1.0",
	}

	if err := valid.Validate(); err != nil {
		t.Fatalf("expected valid event, got error: %v", err)
	}

	tests := []struct {
		name  string
		event Event
	}{
		{"missing source", Event{Type: valid.Type, Timestamp: valid.Timestamp, SchemaVersion: valid.SchemaVersion}},
		{"missing type", Event{Source: valid.Source, Timestamp: valid.Timestamp, SchemaVersion: valid.SchemaVersion}},
		{"missing timestamp", Event{Source: valid.Source, Type: valid.Type, SchemaVersion: valid.SchemaVersion}},
		{"missing schema version", Event{Source: valid.Source, Type: valid.Type, Timestamp: valid.Timestamp}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.event.Validate(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
