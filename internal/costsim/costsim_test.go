package costsim

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/policy"
)

type memoryRepo struct {
	events []domain.Event
	saved  Result
}

func (repo *memoryRepo) IncidentEvents(context.Context, string, int) ([]domain.Event, error) {
	return repo.events, nil
}
func (repo *memoryRepo) SaveCostSimulation(_ context.Context, result Result) error {
	repo.saved = result
	return nil
}

func TestSimulationShowsPolicyVolumeAndSeriesEffect(t *testing.T) {
	start := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	events := []domain.Event{
		{ID: "1", Source: "checkout", Type: "duration", Timestamp: start, SchemaVersion: "1.0", Tags: map[string]string{"request_id": "a", "region": "us"}},
		{ID: "2", Source: "checkout", Type: "duration", Timestamp: start.Add(30 * time.Second), SchemaVersion: "1.0", Tags: map[string]string{"request_id": "b", "region": "us"}},
		{ID: "3", Source: "checkout", Type: "duration", Timestamp: start.Add(60 * time.Second), SchemaVersion: "1.0", Tags: map[string]string{"request_id": "c", "region": "us"}},
	}
	repo := &memoryRepo{events: events}
	active := policy.Policy{Name: "active", Version: "1", DefaultUniqueThreshold: 1000, DefaultAction: policy.ActionAllow, DangerousKeys: []string{"request_id"}, Rules: []policy.Rule{{Dimension: "request_id", UniqueThreshold: 2, Action: policy.ActionDropTag}}}
	result, err := New(repo).Simulate(context.Background(), "INC", active, nil, nil, 100)
	if err != nil {
		t.Fatal(err)
	}
	if result.EventCount != 3 {
		t.Fatalf("events=%d", result.EventCount)
	}
	if result.Active.Series >= result.Baseline.Series {
		t.Fatalf("active series=%d baseline=%d; expected reduction", result.Active.Series, result.Baseline.Series)
	}
	if result.MonthlyActiveCost != nil {
		t.Fatal("dollar cost must remain absent without explicit pricing")
	}
	if repo.saved.ID == "" {
		t.Fatal("expected simulation history to be saved")
	}
}

func TestPricingProducesDollarEstimateOnlyWhenConfigured(t *testing.T) {
	start := time.Now().UTC()
	repo := &memoryRepo{events: []domain.Event{{ID: "1", Source: "api", Type: "metric", Timestamp: start, SchemaVersion: "1.0", Tags: map[string]string{"region": "us"}}}}
	active := policy.Policy{Name: "active", Version: "1", DefaultUniqueThreshold: 1000, DefaultAction: policy.ActionAllow}
	pricing := &Pricing{Name: "reviewed", Currency: "USD", IngestPerGB: 2, ActiveSeriesPer1000Month: 1}
	result, err := New(repo).Simulate(context.Background(), "INC", active, nil, pricing, 100)
	if err != nil {
		t.Fatal(err)
	}
	if result.MonthlyActiveCost == nil {
		t.Fatal("expected explicit pricing to produce dollar estimate")
	}
}

func TestPricingRequiresCurrencyForDollarModel(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "pricing.json")
	payload := `{
	  "name":"missing-currency",
	  "currency":"",
	  "ingest_per_gb":2,
	  "active_series_per_1000_month":0
	}`
	if err := os.WriteFile(filename, []byte(payload), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPricing(filename); err == nil {
		t.Fatal("expected non-zero pricing without currency to fail")
	}
}
