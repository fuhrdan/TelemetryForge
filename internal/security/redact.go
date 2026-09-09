package security

import (
	"encoding/json"
	"strings"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
)

const redactedValue = "[REDACTED]"

// Redactor applies presentation-time redaction to API responses.
//
// Stored Flight Recorder and incident evidence remains full fidelity. This
// boundary is intentionally a read/export control, not destructive storage
// mutation.
type Redactor struct {
	tagKeys       map[string]struct{}
	redactPayload bool
}

// NewRedactor creates a response redactor from a comma-separated tag-key list.
func NewRedactor(tagKeys string, redactPayload bool) *Redactor {
	keys := make(map[string]struct{})
	for _, key := range strings.Split(tagKeys, ",") {
		key = strings.ToLower(strings.TrimSpace(key))
		if key != "" {
			keys[key] = struct{}{}
		}
	}
	return &Redactor{tagKeys: keys, redactPayload: redactPayload}
}

// Event returns a copy safe for API/dashboard presentation.
func (redactor *Redactor) Event(event domain.Event) domain.Event {
	if redactor == nil {
		return event
	}

	copyEvent := event
	if event.Tags != nil {
		copyEvent.Tags = make(map[string]string, len(event.Tags))
		for key, value := range event.Tags {
			if _, sensitive := redactor.tagKeys[strings.ToLower(key)]; sensitive {
				copyEvent.Tags[key] = redactedValue
			} else {
				copyEvent.Tags[key] = value
			}
		}
	}
	if redactor.redactPayload && len(event.Payload) > 0 {
		copyEvent.Payload = json.RawMessage(`{"redacted":true}`)
	}
	return copyEvent
}

// Events redacts a slice without mutating stored/query results.
func (redactor *Redactor) Events(events []domain.Event) []domain.Event {
	if redactor == nil {
		return events
	}
	result := make([]domain.Event, 0, len(events))
	for _, event := range events {
		result = append(result, redactor.Event(event))
	}
	return result
}
