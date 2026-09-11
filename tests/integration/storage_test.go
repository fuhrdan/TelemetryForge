package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/costsim"
	"github.com/fuhrdan/TelemetryForge/internal/domain"
	incidentarchive "github.com/fuhrdan/TelemetryForge/internal/incidentarchive"
	"github.com/fuhrdan/TelemetryForge/internal/policy"
	"github.com/fuhrdan/TelemetryForge/internal/proof"
	"github.com/fuhrdan/TelemetryForge/internal/replay"
	"github.com/fuhrdan/TelemetryForge/internal/router"
	"github.com/fuhrdan/TelemetryForge/internal/schema"
	"github.com/fuhrdan/TelemetryForge/internal/security"
	"github.com/fuhrdan/TelemetryForge/internal/shaping"
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

func TestDistributedCardinalitySharedAcrossTrackers(t *testing.T) {
	databaseURL := os.Getenv("TELEMETRYFORGE_INTEGRATION_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TELEMETRYFORGE_INTEGRATION_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	ctx = security.WithTenant(ctx, "cardinality-integration")

	store, err := storage.NewPostgresStore(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	first := storage.NewDistributedCardinalityTracker(store, "active")
	second := storage.NewDistributedCardinalityTracker(store, "active")
	now := time.Now().UTC()
	source := "cardinality-" + now.Format("150405.000000000")

	for index := 0; index < 12; index++ {
		tracker := first
		if index%2 == 1 {
			tracker = second
		}
		observation, err := tracker.ObserveCardinality(
			ctx,
			"cardinality-integration",
			source,
			"request.duration",
			"request_id",
			fmt.Sprintf("request-%02d", index),
			now.Add(time.Duration(index)*time.Second),
		)
		if err != nil {
			t.Fatal(err)
		}
		if index == 11 && observation.ObservedUnique != 12 {
			t.Fatalf("shared observed=%d, want exact low-cardinality count 12", observation.ObservedUnique)
		}
	}

	states, err := store.ListDistributedCardinalityStates(ctx, "active", 100)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, state := range states {
		if state.Source == source && state.Dimension == "request_id" {
			found = true
			if state.ObservedUnique != 12 {
				t.Fatalf("listed observed=%d, want 12", state.ObservedUnique)
			}
		}
	}
	if !found {
		t.Fatal("expected shared cardinality state in API query")
	}
}

func TestDistributedCardinalityBudgetStatus(t *testing.T) {
	databaseURL := os.Getenv("TELEMETRYFORGE_INTEGRATION_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TELEMETRYFORGE_INTEGRATION_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	ctx = security.WithTenant(ctx, "cardinality-budget-integration")

	store, err := storage.NewPostgresStore(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	active := policy.Policy{
		Name: "integration-budget", Version: "1.2.0",
		DefaultUniqueThreshold: 10000, DefaultAction: policy.ActionAllow,
		Budgets: []policy.Budget{{
			Name: "tenant-hourly-series", Source: "*", Type: "*",
			SeriesLimit: 4, WarningPercent: 50, CriticalPercent: 75,
		}},
	}
	engine, err := policy.NewEngineWithTrackers(
		active, nil, store,
		storage.NewDistributedCardinalityTracker(store, "active"),
		nil, 100,
	)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	for index := 0; index < 4; index++ {
		event := domain.Event{
			ID:       fmt.Sprintf("budget-%d-%d", now.UnixNano(), index),
			TenantID: "cardinality-budget-integration",
			Source:   "checkout-api", Type: "request.duration",
			Timestamp:     now.Add(time.Duration(index) * time.Second),
			SchemaVersion: "1.0",
			Tags:          map[string]string{"region": fmt.Sprintf("region-%d", index)},
		}
		if _, err := engine.EvaluateAt(ctx, event, event.Timestamp); err != nil {
			t.Fatal(err)
		}
	}

	budgets, err := store.ListCardinalityBudgetStatus(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, budget := range budgets {
		if budget.PolicyName == "integration-budget" && budget.BudgetName == "tenant-hourly-series" {
			found = true
			if budget.Status != "exceeded" {
				t.Fatalf("budget status=%q, want exceeded", budget.Status)
			}
		}
	}
	if !found {
		t.Fatal("expected persisted budget status")
	}
}

func TestRoutingOutboxIsolationAndFallback(t *testing.T) {
	databaseURL := os.Getenv("TELEMETRYFORGE_INTEGRATION_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TELEMETRYFORGE_INTEGRATION_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	ctx = security.WithTenant(ctx, "routing-integration")
	store, err := storage.NewPostgresStore(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	now := time.Now().UTC()
	event := domain.Event{
		ID: fmt.Sprintf("routing-%d", now.UnixNano()), TenantID: "routing-integration",
		Source: "checkout-api", Type: "request.error", Timestamp: now,
		SchemaVersion: "1.0", Tags: map[string]string{"severity": "error"},
	}
	plan := []router.Decision{
		{Destination: "primary", RouteRules: []string{"all"}, MaxAttempts: 3, BaseDelayMS: 100, MaxDelayMS: 1000},
		{Destination: "security", RouteRules: []string{"errors"}, MaxAttempts: 1, BaseDelayMS: 100, MaxDelayMS: 1000, FailureFallback: "archive"},
	}
	if err := store.EnqueueRoutingPlan(ctx, event, plan); err != nil {
		t.Fatal(err)
	}
	if err := store.EnqueueRoutingPlan(ctx, event, plan); err != nil {
		t.Fatal(err)
	}

	primary, ok, err := store.ClaimRoutingDelivery(ctx, "primary", 10*time.Second)
	if err != nil || !ok {
		t.Fatalf("claim primary ok=%v err=%v", ok, err)
	}
	if err := store.MarkRoutingDelivered(ctx, primary); err != nil {
		t.Fatal(err)
	}

	securityDelivery, ok, err := store.ClaimRoutingDelivery(ctx, "security", 10*time.Second)
	if err != nil || !ok {
		t.Fatalf("claim security ok=%v err=%v", ok, err)
	}
	fallback := &router.Decision{Destination: "archive", RouteRules: []string{"failure_fallback:security"}, MaxAttempts: 2, BaseDelayMS: 100, MaxDelayMS: 1000}
	if err := store.DeadLetterRoutingDelivery(ctx, securityDelivery, "simulated backend failure", fallback); err != nil {
		t.Fatal(err)
	}

	archive, ok, err := store.ClaimRoutingDelivery(ctx, "archive", 10*time.Second)
	if err != nil || !ok {
		t.Fatalf("claim fallback ok=%v err=%v", ok, err)
	}
	if archive.EventID != event.ID {
		t.Fatalf("fallback event=%q want %q", archive.EventID, event.ID)
	}

	letters, err := store.ListRoutingDeadLetters(ctx, "security", 100)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, letter := range letters {
		if letter.EventID == event.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected per-destination routing dead letter")
	}
}

func TestShapingStatsAndShadowDiffPersistence(t *testing.T) {
	databaseURL := os.Getenv("TELEMETRYFORGE_INTEGRATION_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TELEMETRYFORGE_INTEGRATION_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ctx = security.WithTenant(ctx, "shaping-integration")

	store, err := storage.NewPostgresStore(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	now := time.Now().UTC()
	configName := fmt.Sprintf("integration-shaping-%d", now.UnixNano())
	decision := shaping.Decision{
		TenantID: "shaping-integration", EventID: fmt.Sprintf("shape-%d", now.UnixNano()),
		Source: "checkout-api", EventType: "request.duration", ObservedAt: now,
		ConfigName: configName, ConfigVersion: "1.4.0",
		Rules: []string{"healthy"}, Keep: false, BaseRate: .5, EffectiveRate: .25,
		QueuePressure: .95, OriginalBytes: 1000, ShapedBytes: 700,
		DroppedTags: []string{"debug_id"},
	}
	resolved, reused, err := store.ResolveShapingDecision(ctx, decision)
	if err != nil {
		t.Fatal(err)
	}
	if reused || resolved.EffectiveRate != decision.EffectiveRate {
		t.Fatalf("first decision reused=%v resolved=%#v", reused, resolved)
	}

	changedPressure := decision
	changedPressure.QueuePressure = 0
	changedPressure.EffectiveRate = .5
	resolved, reused, err = store.ResolveShapingDecision(ctx, changedPressure)
	if err != nil {
		t.Fatal(err)
	}
	if !reused || resolved.QueuePressure != .95 || resolved.EffectiveRate != .25 {
		t.Fatalf("retry did not reuse first shaping decision: reused=%v resolved=%#v", reused, resolved)
	}

	stats, err := store.ListShapingStats(ctx, now.Add(-time.Minute), 100)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, stat := range stats {
		if stat.ConfigName == configName && stat.Source == "checkout-api" {
			found = true
			if stat.Observed != 1 || stat.SampledOut != 1 || stat.Transformed != 1 {
				t.Fatalf("unexpected shaping stat: %#v", stat)
			}
		}
	}
	if !found {
		t.Fatal("expected shaping minute aggregate")
	}

	diff := shaping.ShadowDiff{
		TenantID: "shaping-integration", EventID: decision.EventID, ObservedAt: now,
		ActiveConfig: "active", ActiveVersion: "1.4.0", ShadowConfig: "candidate",
		ShadowVersion: "1.4.0-candidate", ActiveKeep: true, ShadowKeep: false,
		ActiveRate: .5, ShadowRate: .2, ActiveEffects: []string{"drop_tag:debug_id"},
		ShadowEffects: []string{"drop_tag:debug_id", "sampled_out"},
	}
	if err := store.RecordShapingShadowDiff(ctx, diff); err != nil {
		t.Fatal(err)
	}
	diffs, err := store.ListShapingShadowDiffs(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	found = false
	for _, item := range diffs {
		if item.EventID == diff.EventID {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected shaping shadow diff")
	}
}

func TestPortableIncidentArchiveRoundTripAcrossTenants(t *testing.T) {
	databaseURL := os.Getenv("TELEMETRYFORGE_INTEGRATION_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TELEMETRYFORGE_INTEGRATION_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	store, err := storage.NewPostgresStore(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	now := time.Now().UTC().Truncate(time.Millisecond)
	sourceTenant := "archive-source-" + now.Format("150405000")
	targetTenant := "archive-target-" + now.Format("150405000")
	sourceIncident := "INC-ARCHIVE-" + now.Format("150405000")
	targetIncident := sourceIncident + "-IMPORTED"

	sourceCtx := security.WithTenant(ctx, sourceTenant)
	event := domain.Event{
		ID:            "archive-event-" + now.Format("150405000"),
		TenantID:      sourceTenant,
		Source:        "checkout-api",
		Type:          "request.error",
		Timestamp:     now,
		SchemaVersion: "1.0",
		CorrelationID: "archive-correlation",
		Tags:          map[string]string{"severity": "error"},
	}
	if err := store.WriteFlightEvent(ctx, event); err != nil {
		t.Fatal(err)
	}
	if _, err := store.FreezeIncident(
		sourceCtx,
		sourceIncident,
		"portable archive integration test",
		now.Add(-time.Minute),
		now.Add(time.Minute),
	); err != nil {
		t.Fatal(err)
	}

	bundle, err := incidentarchive.BuildFromSource(
		sourceCtx,
		store,
		sourceIncident,
		100,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	filename := filepath.Join(t.TempDir(), "incident.tfincident")
	manifest, err := incidentarchive.WriteFile(filename, bundle, nil)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := incidentarchive.ReadFile(filename, nil)
	if err != nil {
		t.Fatal(err)
	}
	if verified.Manifest.ArchiveID != manifest.ArchiveID {
		t.Fatal("archive ID changed during round trip")
	}

	targetCtx := security.WithTenant(ctx, targetTenant)
	importedIncident := verified.Incident
	importedIncident.ID = targetIncident
	records := storage.NormalizeImportedEvents(verified.Events, targetTenant)

	provenance := incidentarchive.ImportProvenance{
		ArchiveID:             verified.Manifest.ArchiveID,
		SourceTenantID:        verified.Manifest.TenantID,
		SourceIncidentID:      verified.Manifest.IncidentID,
		ImportedIncidentID:    targetIncident,
		FormatVersion:         verified.Manifest.FormatVersion,
		TelemetryForgeVersion: verified.Manifest.TelemetryForge,
		FileSHA256:            "integration-file-hash",
		Encrypted:             false,
	}
	inserted, err := store.ImportIncidentArchive(
		targetCtx,
		importedIncident,
		records,
		provenance,
	)
	if err != nil {
		t.Fatal(err)
	}
	if inserted != 1 {
		t.Fatalf("inserted=%d, want 1", inserted)
	}

	importedEvents, err := store.IncidentEvents(targetCtx, targetIncident, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(importedEvents) != 1 {
		t.Fatalf("imported event count=%d, want 1", len(importedEvents))
	}
	if importedEvents[0].TenantID != targetTenant {
		t.Fatalf("imported tenant=%q, want %q", importedEvents[0].TenantID, targetTenant)
	}

	imports, err := store.ListIncidentArchiveImports(targetCtx, 20)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range imports {
		if item.ArchiveID == manifest.ArchiveID &&
			item.ImportedIncidentID == targetIncident {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected archive import provenance row")
	}

	// Re-importing the same archive into the same tenant is rejected so audit
	// provenance cannot silently point at only one of multiple imports.
	secondIncident := importedIncident
	secondIncident.ID = targetIncident + "-SECOND"
	provenance.ImportedIncidentID = secondIncident.ID
	if _, err := store.ImportIncidentArchive(
		targetCtx,
		secondIncident,
		records,
		provenance,
	); err == nil {
		t.Fatal("duplicate archive import should fail")
	}
}

func TestOperationalProofPersistence(t *testing.T) {
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
	runID := fmt.Sprintf("proof-%d", now.UnixNano())
	run := proof.Run{Format: proof.Format, FormatVersion: proof.Version, RunID: runID, GitCommit: "integration", Scenario: "storage-proof", Status: "pass", StartedAt: now, CompletedAt: now.Add(time.Second), Assertions: []proof.Assertion{{Name: "stored", Passed: true}}, Evidence: []proof.Evidence{{Kind: "integration", Reference: "database"}}, Configuration: []proof.Fingerprint{{Name: "integration", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}}
	sha := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if err := store.RecordOperationalProof(ctx, run, sha, 123); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordOperationalProof(ctx, run, sha, 123); err != nil {
		t.Fatalf("idempotent record failed: %v", err)
	}
	if err := store.RecordOperationalProof(ctx, run, "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc", 124); err == nil {
		t.Fatal("expected run-id provenance conflict")
	}
	items, err := store.ListOperationalProofs(ctx, 200)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range items {
		if item.Run.RunID == runID {
			found = true
			if item.ArtifactSHA256 != sha || item.ArtifactBytes != 123 {
				t.Fatalf("bad provenance: %#v", item)
			}
			break
		}
	}
	if !found {
		t.Fatal("recorded proof not found")
	}
}
