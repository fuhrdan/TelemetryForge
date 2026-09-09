// Package costsim estimates telemetry volume and series pressure before policy
// changes are promoted. Dollar estimates are emitted only when the operator
// supplies an explicit pricing model.
package costsim

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/policy"
	"github.com/fuhrdan/TelemetryForge/internal/worker"
)

const secondsPerMonth = 30 * 24 * 60 * 60

// Repository is the durable simulation-history contract.
type Repository interface {
	IncidentEvents(context.Context, string, int) ([]domain.Event, error)
	SaveCostSimulation(context.Context, Result) error
}

// Pricing is operator-supplied. Zero prices keep the simulator volume-only.
type Pricing struct {
	Name                     string  `json:"name"`
	Currency                 string  `json:"currency"`
	IngestPerGB              float64 `json:"ingest_per_gb"`
	ActiveSeriesPer1000Month float64 `json:"active_series_per_1000_month"`
	Notes                    string  `json:"notes,omitempty"`
}

// Profile describes one policy's effect on the incident sample.
type Profile struct {
	Bytes         int64 `json:"bytes"`
	Series        int64 `json:"series"`
	ChangedEvents int64 `json:"changed_events"`
}

// Result is a transparent projection from a frozen incident window.
type Result struct {
	ID                  string         `json:"simulation_id"`
	IncidentID          string         `json:"incident_id"`
	ActivePolicy        string         `json:"active_policy"`
	ActiveVersion       string         `json:"active_version"`
	ShadowPolicy        string         `json:"shadow_policy,omitempty"`
	ShadowVersion       string         `json:"shadow_version,omitempty"`
	PricingModel        string         `json:"pricing_model,omitempty"`
	Currency            string         `json:"currency,omitempty"`
	StartedAt           time.Time      `json:"started_at"`
	CompletedAt         time.Time      `json:"completed_at"`
	WindowSeconds       float64        `json:"window_seconds"`
	EventCount          int64          `json:"event_count"`
	Baseline            Profile        `json:"baseline"`
	Active              Profile        `json:"active"`
	Shadow              Profile        `json:"shadow"`
	MonthlyBaselineGB   float64        `json:"projected_monthly_baseline_gb"`
	MonthlyActiveGB     float64        `json:"projected_monthly_active_gb"`
	MonthlyShadowGB     float64        `json:"projected_monthly_shadow_gb"`
	MonthlyBaselineCost *float64       `json:"projected_monthly_baseline_cost,omitempty"`
	MonthlyActiveCost   *float64       `json:"projected_monthly_active_cost,omitempty"`
	MonthlyShadowCost   *float64       `json:"projected_monthly_shadow_cost,omitempty"`
	Assumptions         map[string]any `json:"assumptions"`
	Error               string         `json:"error,omitempty"`
}

// Simulator evaluates baseline, active, and shadow representations.
type Simulator struct{ repository Repository }

func New(repository Repository) *Simulator { return &Simulator{repository: repository} }

// Simulate projects the observed incident sample to a 30-day month.
func (simulator *Simulator) Simulate(
	ctx context.Context,
	incidentID string,
	active policy.Policy,
	shadow *policy.Policy,
	pricing *Pricing,
	maxEvents int,
) (result Result, retErr error) {
	if strings.TrimSpace(incidentID) == "" {
		return Result{}, fmt.Errorf("incident id is required")
	}
	if err := active.Validate(); err != nil {
		return Result{}, fmt.Errorf("active policy: %w", err)
	}
	if shadow != nil {
		if err := shadow.Validate(); err != nil {
			return Result{}, fmt.Errorf("shadow policy: %w", err)
		}
	}
	if maxEvents < 1 || maxEvents > 100000 {
		maxEvents = 10000
	}

	id, err := simulationID()
	if err != nil {
		return Result{}, err
	}
	result = Result{
		ID:            id,
		IncidentID:    incidentID,
		ActivePolicy:  active.Name,
		ActiveVersion: active.Version,
		StartedAt:     time.Now().UTC(),
		Assumptions: map[string]any{
			"projection_window_days": 30,
			"series_count":           "exact distinct series within frozen incident sample",
			"series_pricing":         "sample distinct series are treated as representative active monthly series; series are not linearly extrapolated",
			"volume_projection":      "canonical JSON sample bytes scaled linearly by incident event-time window",
			"transport_encoding":     "does not model backend compression, protocol framing, aggregation, retention tiers, or vendor-specific accounting",
			"warning":                "projection is directional, not a vendor bill or capacity guarantee",
		},
	}
	if shadow != nil {
		result.ShadowPolicy = shadow.Name
		result.ShadowVersion = shadow.Version
	}
	if pricing != nil {
		result.PricingModel = pricing.Name
		result.Currency = pricing.Currency
	}

	defer func() {
		result.CompletedAt = time.Now().UTC()
		if retErr != nil {
			result.Error = retErr.Error()
		}
		if err := simulator.repository.SaveCostSimulation(context.WithoutCancel(ctx), result); err != nil && retErr == nil {
			retErr = fmt.Errorf("save cost simulation: %w", err)
		}
	}()

	events, err := simulator.repository.IncidentEvents(ctx, incidentID, maxEvents)
	if err != nil {
		return result, fmt.Errorf("load incident events: %w", err)
	}
	if len(events) == 0 {
		return result, fmt.Errorf("incident %q contains no events", incidentID)
	}

	normalizer := worker.Normalizer{}
	activeEngine, err := policy.NewEngine(active, nil, nil, 20000)
	if err != nil {
		return result, err
	}
	var shadowEngine *policy.Engine
	if shadow != nil {
		shadowEngine, err = policy.NewEngine(*shadow, nil, nil, 20000)
		if err != nil {
			return result, err
		}
	}

	baselineSeries := make(map[[32]byte]struct{})
	activeSeries := make(map[[32]byte]struct{})
	shadowSeries := make(map[[32]byte]struct{})

	minTime := events[0].Timestamp
	maxTime := events[0].Timestamp

	for _, event := range events {
		normalized, err := normalizer.Process(ctx, event)
		if err != nil {
			return result, err
		}
		if event.Timestamp.Before(minTime) {
			minTime = event.Timestamp
		}
		if event.Timestamp.After(maxTime) {
			maxTime = event.Timestamp
		}

		baselineBytes, _ := json.Marshal(normalized)
		result.Baseline.Bytes += int64(len(baselineBytes))
		baselineSeries[seriesHash(normalized)] = struct{}{}

		activeEvent, err := activeEngine.EvaluateAt(ctx, normalized, normalized.Timestamp)
		if err != nil {
			return result, err
		}
		activeBytes, _ := json.Marshal(activeEvent)
		result.Active.Bytes += int64(len(activeBytes))
		activeSeries[seriesHash(activeEvent)] = struct{}{}
		if eventChanged(normalized, activeEvent) {
			result.Active.ChangedEvents++
		}

		if shadowEngine != nil {
			shadowEvent, err := shadowEngine.EvaluateAt(ctx, normalized, normalized.Timestamp)
			if err != nil {
				return result, err
			}
			shadowBytes, _ := json.Marshal(shadowEvent)
			result.Shadow.Bytes += int64(len(shadowBytes))
			shadowSeries[seriesHash(shadowEvent)] = struct{}{}
			if eventChanged(normalized, shadowEvent) {
				result.Shadow.ChangedEvents++
			}
		} else {
			result.Shadow.Bytes += int64(len(baselineBytes))
			shadowSeries[seriesHash(normalized)] = struct{}{}
		}
		result.EventCount++
	}

	result.Baseline.Series = int64(len(baselineSeries))
	result.Active.Series = int64(len(activeSeries))
	result.Shadow.Series = int64(len(shadowSeries))

	window := maxTime.Sub(minTime).Seconds()
	if window < 1 {
		window = 1
	}
	result.WindowSeconds = window
	scale := secondsPerMonth / window
	result.MonthlyBaselineGB = bytesToGB(float64(result.Baseline.Bytes) * scale)
	result.MonthlyActiveGB = bytesToGB(float64(result.Active.Bytes) * scale)
	result.MonthlyShadowGB = bytesToGB(float64(result.Shadow.Bytes) * scale)

	if pricing != nil && (pricing.IngestPerGB > 0 || pricing.ActiveSeriesPer1000Month > 0) {
		baseline := result.MonthlyBaselineGB*pricing.IngestPerGB + float64(result.Baseline.Series)/1000*pricing.ActiveSeriesPer1000Month
		activeCost := result.MonthlyActiveGB*pricing.IngestPerGB + float64(result.Active.Series)/1000*pricing.ActiveSeriesPer1000Month
		shadowCost := result.MonthlyShadowGB*pricing.IngestPerGB + float64(result.Shadow.Series)/1000*pricing.ActiveSeriesPer1000Month
		result.MonthlyBaselineCost = &baseline
		result.MonthlyActiveCost = &activeCost
		result.MonthlyShadowCost = &shadowCost
	}

	return result, nil
}

func eventChanged(before, after domain.Event) bool {
	if policy.IsQuarantined(after) {
		return true
	}
	if len(before.Tags) != len(after.Tags) {
		return true
	}
	for key, value := range before.Tags {
		if after.Tags[key] != value {
			return true
		}
	}
	return false
}

func seriesHash(event domain.Event) [32]byte {
	keys := make([]string, 0, len(event.Tags))
	for key := range event.Tags {
		if strings.HasPrefix(key, "telemetryforge.") {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	builder := strings.Builder{}
	builder.WriteString(event.Source)
	builder.WriteByte('|')
	builder.WriteString(event.Type)
	for _, key := range keys {
		builder.WriteByte('|')
		builder.WriteString(key)
		builder.WriteByte('=')
		builder.WriteString(event.Tags[key])
	}
	return sha256.Sum256([]byte(builder.String()))
}

func bytesToGB(value float64) float64 { return value / (1024 * 1024 * 1024) }

func simulationID() (string, error) {
	raw := make([]byte, 6)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return fmt.Sprintf("COST-%d-%s", time.Now().UTC().UnixMilli(), hex.EncodeToString(raw)), nil
}

// LoadPricing reads a strict operator-supplied pricing model.
func LoadPricing(filename string) (Pricing, error) {
	payload, err := os.ReadFile(filename)
	if err != nil {
		return Pricing{}, fmt.Errorf("read pricing %q: %w", filename, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var pricing Pricing
	if err := decoder.Decode(&pricing); err != nil {
		return Pricing{}, fmt.Errorf("decode pricing %q: %w", filename, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return Pricing{}, fmt.Errorf("pricing %q contains trailing JSON", filename)
		}
		return Pricing{}, fmt.Errorf("decode pricing trailing data: %w", err)
	}
	if strings.TrimSpace(pricing.Name) == "" {
		return Pricing{}, fmt.Errorf("pricing name is required")
	}
	if pricing.IngestPerGB < 0 || pricing.ActiveSeriesPer1000Month < 0 {
		return Pricing{}, fmt.Errorf("pricing values cannot be negative")
	}
	if (pricing.IngestPerGB > 0 || pricing.ActiveSeriesPer1000Month > 0) &&
		strings.TrimSpace(pricing.Currency) == "" {
		return Pricing{}, fmt.Errorf("pricing currency is required when non-zero prices are configured")
	}
	return pricing, nil
}
