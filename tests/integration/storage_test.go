package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/storage"
)

func TestTimescalePersistenceIsIdempotent(t *testing.T) {
	databaseURL := os.Getenv("TELEMETRYFORGE_INTEGRATION_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TELEMETRYFORGE_INTEGRATION_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	store, err := storage.NewPostgresStore(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	value := 42.5
	event := domain.Event{
		ID:            "integration-idempotency-event",
		Source:        "integration-test",
		Type:          "metric.sample",
		Timestamp:     time.Now().UTC().Truncate(time.Millisecond),
		Value:         &value,
		Unit:          "ms",
		SchemaVersion: "1.0",
	}

	// At-least-once delivery may invoke persistence twice. Both calls must
	// succeed, while only one stored event should remain.
	if err := store.WriteEvent(ctx, event); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteEvent(ctx, event); err != nil {
		t.Fatal(err)
	}

	events, err := store.QueryEvents(ctx, storage.Query{
		Source: event.Source,
		Type:   event.Type,
		Limit:  10,
	})
	if err != nil {
		t.Fatal(err)
	}

	count := 0
	for _, stored := range events {
		if stored.ID == event.ID {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("found %d stored rows for duplicate event, want 1", count)
	}
}

func TestFlightRecorderFreeze(t *testing.T) {
	databaseURL := os.Getenv("TELEMETRYFORGE_INTEGRATION_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TELEMETRYFORGE_INTEGRATION_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	store, err := storage.NewPostgresStore(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	now := time.Now().UTC()
	event := domain.Event{
		ID:            "integration-flight-event",
		Source:        "integration-test",
		Type:          "incident.sample",
		Timestamp:     now,
		SchemaVersion: "1.0",
	}
	if err := store.WriteFlightEvent(ctx, event); err != nil {
		t.Fatal(err)
	}

	count, err := store.FreezeIncident(
		ctx,
		"integration-incident",
		"Integration Flight Recorder test",
		now.Add(-time.Minute),
		now.Add(time.Minute),
	)
	if err != nil {
		t.Fatal(err)
	}
	if count < 1 {
		t.Fatalf("froze %d events, want at least 1", count)
	}
}
