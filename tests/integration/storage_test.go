package integration

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/costsim"
	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/policy"
	"github.com/fuhrdan/TelemetryForge/internal/replay"
	"github.com/fuhrdan/TelemetryForge/internal/schema"
	"github.com/fuhrdan/TelemetryForge/internal/security"
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

func TestFlightRecorderFreezeDeduplicatesRetries(t *testing.T) {
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
		ID:            "integration-flight-retry-event",
		Source:        "integration-test",
		Type:          "incident.retry",
		Timestamp:     now,
		SchemaVersion: "1.0",
	}

	// The rolling recorder may see the same canonical event on an at-least-once
	// processing retry. The durable incident timeline should still contain one
	// canonical event, not two processing attempts.
	if err := store.WriteFlightEvent(ctx, event); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteFlightEvent(ctx, event); err != nil {
		t.Fatal(err)
	}

	count, err := store.FreezeIncident(
		ctx,
		"integration-incident-dedup",
		"Integration Flight Recorder retry dedup test",
		now.Add(-time.Minute),
		now.Add(time.Minute),
	)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("froze %d canonical events, want 1", count)
	}

	events, err := store.IncidentEvents(ctx, "integration-incident-dedup", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].ID != event.ID {
		t.Fatalf("unexpected incident events: %#v", events)
	}
}

func TestPolicyEvidencePersistence(t *testing.T) {
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
	finding := policy.Finding{
		PolicyName:       "integration-active",
		PolicyVersion:    "1",
		Mode:             "active",
		Source:           "integration-policy",
		EventType:        "request.duration",
		Dimension:        "request_id",
		ObservedUnique:   100,
		ProjectedUnique:  1000,
		Action:           policy.ActionDropTag,
		Reason:           "integration finding",
		ValueFingerprint: "abcdef123456",
		FirstSeen:        now.Add(-time.Minute),
		LastSeen:         now,
	}
	if err := store.RecordCardinalityFinding(ctx, finding); err != nil {
		t.Fatal(err)
	}

	diff := policy.Diff{
		ObservedAt:    now,
		Source:        finding.Source,
		EventType:     finding.EventType,
		Dimension:     finding.Dimension,
		ActivePolicy:  "integration-active",
		ActiveVersion: "1",
		ActiveAction:  policy.ActionAllow,
		ShadowPolicy:  "integration-shadow",
		ShadowVersion: "2",
		ShadowAction:  policy.ActionDropTag,
		Reason:        "integration diff",
	}
	if err := store.RecordPolicyDiff(ctx, diff); err != nil {
		t.Fatal(err)
	}

	findings, err := store.ListCardinalityFindings(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range findings {
		if item.PolicyName == finding.PolicyName && item.Source == finding.Source {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected persisted cardinality finding")
	}

	diffs, err := store.ListPolicyDiffs(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	found = false
	for _, item := range diffs {
		if item.ActivePolicy == diff.ActivePolicy && item.ShadowPolicy == diff.ShadowPolicy {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected persisted shadow-policy diff")
	}
}

func TestReplayAndCostHistoryPersistence(t *testing.T) {
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
	completed := now.Add(time.Second)
	run := replay.Run{
		ID:            "integration-replay-" + now.Format("20060102T150405.000000000"),
		IncidentID:    "integration-incident",
		Mode:          "analysis",
		Status:        "running",
		ActivePolicy:  "integration-active",
		ActiveVersion: "1",
		StartedAt:     now,
	}
	if err := store.StartReplay(ctx, run); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordReplayEvent(ctx, replay.EventResult{
		RunID:           run.ID,
		EventID:         "integration-event",
		Changed:         true,
		DroppedTagCount: 1,
		FindingCount:    1,
	}); err != nil {
		t.Fatal(err)
	}

	run.Status = "completed"
	run.CompletedAt = &completed
	run.EventCount = 1
	run.ChangedEventCount = 1
	run.DroppedTagCount = 1
	run.FindingCount = 1
	if err := store.CompleteReplay(ctx, run); err != nil {
		t.Fatal(err)
	}

	runs, err := store.ListReplayRuns(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	foundRun := false
	for _, item := range runs {
		if item.ID == run.ID && item.Status == "completed" {
			foundRun = true
			break
		}
	}
	if !foundRun {
		t.Fatal("expected completed replay run in history")
	}

	simulation := costsim.Result{
		ID:                "integration-cost-" + now.Format("20060102T150405.000000000"),
		IncidentID:        "integration-incident",
		ActivePolicy:      "integration-active",
		ActiveVersion:     "1",
		StartedAt:         now,
		CompletedAt:       completed,
		WindowSeconds:     60,
		EventCount:        10,
		Baseline:          costsim.Profile{Bytes: 1000, Series: 10},
		Active:            costsim.Profile{Bytes: 750, Series: 5, ChangedEvents: 5},
		Shadow:            costsim.Profile{Bytes: 700, Series: 4, ChangedEvents: 6},
		MonthlyBaselineGB: 1.0,
		MonthlyActiveGB:   0.75,
		MonthlyShadowGB:   0.70,
		Assumptions:       map[string]any{"integration": true},
	}
	if err := store.SaveCostSimulation(ctx, simulation); err != nil {
		t.Fatal(err)
	}

	simulations, err := store.ListCostSimulations(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	foundSimulation := false
	for _, item := range simulations {
		if item.ID == simulation.ID && item.Active.Series == 5 {
			foundSimulation = true
			break
		}
	}
	if !foundSimulation {
		t.Fatal("expected cost simulation in history")
	}
}

func TestTenantIsolationAllowsSameEventIDWithoutCrossRead(t *testing.T) {
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

	eventID := "tenant-shared-" + time.Now().UTC().Format("20060102T150405.000000000")
	alpha := domain.Event{
		ID: eventID, TenantID: "alpha", Source: "alpha-source", Type: "tenant.test",
		Timestamp: time.Now().UTC(), SchemaVersion: "1.0",
	}
	beta := alpha
	beta.TenantID = "beta"
	beta.Source = "beta-source"

	if err := store.WriteEvent(ctx, alpha); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteEvent(ctx, beta); err != nil {
		t.Fatal(err)
	}

	alphaCtx := security.WithTenant(ctx, "alpha")
	alphaEvents, err := store.QueryEvents(alphaCtx, storage.Query{Limit: 1000})
	if err != nil {
		t.Fatal(err)
	}
	betaCtx := security.WithTenant(ctx, "beta")
	betaEvents, err := store.QueryEvents(betaCtx, storage.Query{Limit: 1000})
	if err != nil {
		t.Fatal(err)
	}

	find := func(events []domain.Event, id string) *domain.Event {
		for index := range events {
			if events[index].ID == id {
				return &events[index]
			}
		}
		return nil
	}

	alphaFound := find(alphaEvents, eventID)
	betaFound := find(betaEvents, eventID)
	if alphaFound == nil || alphaFound.Source != "alpha-source" {
		t.Fatalf("alpha tenant result=%#v", alphaFound)
	}
	if betaFound == nil || betaFound.Source != "beta-source" {
		t.Fatalf("beta tenant result=%#v", betaFound)
	}
}

func TestTenantIsolationAllowsSameIncidentID(t *testing.T) {
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
	incidentID := "shared-incident-" + now.Format("20060102T150405.000000000")

	for _, tenant := range []string{"alpha", "beta"} {
		event := domain.Event{
			ID:            tenant + "-flight-" + now.Format("150405.000000000"),
			TenantID:      tenant,
			Source:        tenant + "-source",
			Type:          "request.error",
			Timestamp:     now,
			SchemaVersion: "1.0",
		}
		if err := store.WriteFlightEvent(ctx, event); err != nil {
			t.Fatal(err)
		}

		tenantCtx := security.WithTenant(ctx, tenant)
		count, err := store.FreezeIncident(
			tenantCtx,
			incidentID,
			tenant+" incident",
			now.Add(-time.Minute),
			now.Add(time.Minute),
		)
		if err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("tenant %s froze %d events, want 1", tenant, count)
		}
	}

	for _, tenant := range []string{"alpha", "beta"} {
		events, err := store.IncidentEvents(
			security.WithTenant(ctx, tenant),
			incidentID,
			10,
		)
		if err != nil {
			t.Fatal(err)
		}
		if len(events) != 1 || events[0].TenantID != tenant {
			t.Fatalf("tenant %s incident events=%#v", tenant, events)
		}
	}
}

func TestSchemaIntelligenceIdempotencyAndBreakingDrift(t *testing.T) {
	databaseURL := os.Getenv("TELEMETRYFORGE_INTEGRATION_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TELEMETRYFORGE_INTEGRATION_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	ctx = security.WithTenant(ctx, "schema-integration")

	store, err := storage.NewPostgresStore(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	stamp := time.Now().UTC()
	source := "orders-api-" + stamp.Format("150405.000000000")
	var duplicate domain.Event
	for index := 0; index < 24; index++ {
		event := domain.Event{
			ID:            "schema-v1-" + stamp.Format("150405.000000000") + "-" + string(rune('a'+index)),
			TenantID:      "schema-integration",
			Source:        source,
			Type:          "order.created",
			Timestamp:     stamp.Add(time.Duration(index) * time.Millisecond),
			SchemaVersion: "1.0",
			SchemaURL:     "https://opentelemetry.io/schemas/1.44.0",
			Payload:       json.RawMessage(`{"order_id":"o-1","total":42.5,"currency":"USD"}`),
		}
		if index == 0 {
			duplicate = event
		}
		if _, _, err := store.ObserveSchema(ctx, event, schema.Describe(event)); err != nil {
			t.Fatal(err)
		}
	}

	// Same at-least-once event must be a no-op.
	if _, _, err := store.ObserveSchema(ctx, duplicate, schema.Describe(duplicate)); err != nil {
		t.Fatal(err)
	}

	drift := domain.Event{
		ID:            "schema-drift-" + stamp.Format("150405.000000000"),
		TenantID:      "schema-integration",
		Source:        source,
		Type:          "order.created",
		Timestamp:     stamp.Add(time.Second),
		SchemaVersion: "1.0",
		SchemaURL:     "https://opentelemetry.io/schemas/1.44.0",
		Payload:       json.RawMessage(`{"total":"42.50","currency":"USD"}`),
	}
	if _, _, err := store.ObserveSchema(ctx, drift, schema.Describe(drift)); err != nil {
		t.Fatal(err)
	}

	history, err := store.SchemaHistory(ctx, source, "order.created")
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 {
		t.Fatalf("history=%d, want 1", len(history))
	}
	if history[0].ObservationCount != 25 {
		t.Fatalf("observations=%d, want 25 (duplicate must not count)", history[0].ObservationCount)
	}
	if history[0].Health != "breaking" {
		t.Fatalf("health=%q, want breaking", history[0].Health)
	}

	drifts, err := store.ListSchemaDrifts(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	foundType := false
	foundMissing := false
	for _, finding := range drifts {
		if finding.Source != source {
			continue
		}
		if finding.Kind == "type_changed" {
			foundType = true
		}
		if finding.Kind == "required_field_missing" && finding.Path == "payload.order_id" {
			foundMissing = true
		}
	}
	if !foundType || !foundMissing {
		t.Fatalf("type=%t missing=%t drifts=%#v", foundType, foundMissing, drifts)
	}

	versionTwo := domain.Event{
		ID:            "schema-v2-" + stamp.Format("150405.000000000"),
		TenantID:      "schema-integration",
		Source:        source,
		Type:          "order.created",
		Timestamp:     stamp.Add(2 * time.Second),
		SchemaVersion: "2.0",
		SchemaURL:     "https://opentelemetry.io/schemas/1.44.0",
		Payload:       json.RawMessage(`{"total":"50.00","currency":"USD"}`),
	}
	if _, _, err := store.ObserveSchema(ctx, versionTwo, schema.Describe(versionTwo)); err != nil {
		t.Fatal(err)
	}

	diff, err := store.SchemaDiff(ctx, source, "order.created", "1.0", "2.0")
	if err != nil {
		t.Fatal(err)
	}
	if diff.Compatibility != "breaking" {
		t.Fatalf("compatibility=%q", diff.Compatibility)
	}
}
