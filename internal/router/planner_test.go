package router

import (
	"context"
	"testing"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/connectors"
	"github.com/fuhrdan/TelemetryForge/internal/domain"
)

type memoryPlanStore struct {
	plans []Decision
	diffs []ShadowDiff
}

func (s *memoryPlanStore) EnqueueRoutingPlan(_ context.Context, _ domain.Event, d []Decision) error {
	s.plans = append(s.plans, d...)
	return nil
}
func (s *memoryPlanStore) RecordRoutingShadowDiff(_ context.Context, d ShadowDiff) error {
	s.diffs = append(s.diffs, d)
	return nil
}

func testConfig() Config {
	return Config{
		Name: "active", Version: "1",
		Destinations: []Destination{
			{Name: "primary", Type: DestinationKafka, Enabled: true, Topic: "telemetry.routed.primary"},
			{Name: "security", Type: DestinationKafka, Enabled: true, Topic: "telemetry.routed.security"},
			{Name: "fallback", Type: DestinationKafka, Enabled: true, Topic: "telemetry.routed.fallback"},
		},
		FallbackDestination: "primary",
		Rules: []Rule{
			{Name: "errors", Tenant: "acme", Source: "checkout-*", Type: "*", Severity: []string{"error", "critical"}, Tags: map[string]string{"environment": "prod*"}, Destinations: []string{"primary", "security"}},
		},
	}
}

func TestPlanMatchesTenantSourceTypeSeverityAndTags(t *testing.T) {
	config := testConfig()
	event := domain.Event{TenantID: "acme", Source: "checkout-api", Type: "request.error", Tags: map[string]string{"severity": "error", "environment": "production"}}
	plan := config.Plan(event)
	if len(plan) != 2 || plan[0].Destination != "primary" || plan[1].Destination != "security" {
		t.Fatalf("plan=%#v", plan)
	}
}

func TestPlanUsesFallbackWhenNoRuleMatches(t *testing.T) {
	plan := testConfig().Plan(domain.Event{TenantID: "other", Source: "api", Type: "request", Tags: map[string]string{}})
	if len(plan) != 1 || plan[0].Destination != "primary" || plan[0].RouteRules[0] != "fallback" {
		t.Fatalf("plan=%#v", plan)
	}
}

func TestFanoutDeduplicatesDestinationAcrossRules(t *testing.T) {
	config := testConfig()
	config.Rules = append(config.Rules, Rule{Name: "all-prod", Tenant: "acme", Tags: map[string]string{"environment": "prod*"}, Destinations: []string{"primary"}})
	plan := config.Plan(domain.Event{TenantID: "acme", Source: "checkout-api", Type: "request.error", Tags: map[string]string{"severity": "error", "environment": "production"}})
	if len(plan) != 2 {
		t.Fatalf("destinations=%d, want 2", len(plan))
	}
	if len(plan[0].RouteRules) != 2 {
		t.Fatalf("primary rules=%#v", plan[0].RouteRules)
	}
}

func TestShadowPlanNeverEnqueuesShadowDestinations(t *testing.T) {
	active := testConfig()
	shadow := testConfig()
	shadow.Name = "shadow"
	shadow.Version = "2"
	shadow.Rules[0].Destinations = []string{"security"}
	store := &memoryPlanStore{}
	planner, err := NewPlanner(active, &shadow, store)
	if err != nil {
		t.Fatal(err)
	}
	event := domain.Event{ID: "evt", TenantID: "acme", Source: "checkout-api", Type: "request.error", Tags: map[string]string{"severity": "error", "environment": "production"}}
	if err := planner.Process(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if len(store.plans) != 2 {
		t.Fatalf("active plans=%d, want 2", len(store.plans))
	}
	for _, decision := range store.plans {
		if decision.Destination != "primary" && decision.Destination != "security" {
			t.Fatalf("unexpected active destination %#v", decision)
		}
	}
	if len(store.diffs) != 1 || len(store.diffs[0].Removed) != 1 || store.diffs[0].Removed[0] != "primary" {
		t.Fatalf("diff=%#v", store.diffs)
	}
}

func TestConfigRejectsFallbackCycle(t *testing.T) {
	config := Config{Name: "bad", Version: "1", Destinations: []Destination{
		{Name: "a", Type: DestinationKafka, Enabled: true, Topic: "a", FailureFallback: "b"},
		{Name: "b", Type: DestinationKafka, Enabled: true, Topic: "b", FailureFallback: "a"},
	}}
	if err := config.Validate(); err == nil {
		t.Fatal("expected fallback cycle failure")
	}
}

func TestComparePlansUsesStableSets(t *testing.T) {
	event := domain.Event{ID: "evt", TenantID: "t"}
	active := Config{Name: "a", Version: "1"}
	shadow := Config{Name: "s", Version: "2"}
	diff, changed := ComparePlans(event, active, shadow, []Decision{{Destination: "b"}, {Destination: "a"}}, []Decision{{Destination: "b"}, {Destination: "c"}}, time.Now())
	if !changed || len(diff.Added) != 1 || diff.Added[0] != "c" || len(diff.Removed) != 1 || diff.Removed[0] != "a" {
		t.Fatalf("diff=%#v", diff)
	}
}

func TestConfigRejectsStaticSensitiveHTTPHeader(t *testing.T) {
	config := Config{Name: "bad", Version: "1", Destinations: []Destination{{
		Name: "webhook", Type: DestinationHTTP, Enabled: true, URL: "https://example.invalid",
		Headers: map[string]string{"X-API-Key": "do-not-commit"},
	}}}
	if err := config.Validate(); err == nil {
		t.Fatal("expected static secret header rejection")
	}
}

func TestConfigAllowsSecretBackedHTTPHeader(t *testing.T) {
	config := Config{Name: "ok", Version: "1", Destinations: []Destination{{
		Name: "webhook", Type: DestinationHTTP, Enabled: true, URL: "https://example.invalid",
		HeaderEnv: map[string]string{"X-API-Key": "TELEMETRYFORGE_WEBHOOK_API_KEY"},
	}}}
	if err := config.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestConnectorDestinationValidatesAndLegacyDestinationsRemainCompatible(t *testing.T) {
	config := testConfig()
	config.Destinations = append(config.Destinations, Destination{Name: "otlp", Type: DestinationConnector, Enabled: true, Connector: &connectors.Spec{Kind: connectors.KindOTLPHTTP, Endpoint: "https://collector.example.invalid"}})
	if err := config.Validate(); err != nil {
		t.Fatal(err)
	}
	legacy, err := ConnectorSpec(config.Destinations[0])
	if err != nil {
		t.Fatal(err)
	}
	if legacy.Kind != connectors.KindKafka {
		t.Fatalf("legacy Kafka kind=%q", legacy.Kind)
	}
}
