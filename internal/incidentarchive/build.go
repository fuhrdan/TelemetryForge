package incidentarchive

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/changeintel"
	"github.com/fuhrdan/TelemetryForge/internal/costsim"
	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/evidence"
	"github.com/fuhrdan/TelemetryForge/internal/replay"
	"github.com/fuhrdan/TelemetryForge/internal/schema"
)

// Source is the tenant-scoped read contract required to assemble an archive.
type Source interface {
	IncidentArchiveData(
		ctx context.Context,
		incidentID string,
		limit int,
	) (IncidentMetadata, []EventRecord, error)
	ListReplayRuns(ctx context.Context, limit int) ([]replay.Run, error)
	ListCostSimulations(ctx context.Context, limit int) ([]costsim.Result, error)
	SchemaHistory(ctx context.Context, source, eventType string) ([]schema.RegistryEntry, error)
	ListSchemaDrifts(ctx context.Context, limit int) ([]schema.Drift, error)
}

type changeSource interface {
	ChangesBetween(context.Context, time.Time, time.Time, int) ([]changeintel.Marker, error)
}

// BuildFromSource assembles a portable bundle from one frozen incident.
//
// Configuration snapshots are supplied explicitly by the caller because policy,
// shaping, and routing are versioned files rather than database-owned state in
// the current architecture.
func BuildFromSource(
	ctx context.Context,
	source Source,
	incidentID string,
	maxEvents int,
	configurations []ConfigurationSnapshot,
) (Bundle, error) {
	incident, records, err := source.IncidentArchiveData(ctx, incidentID, maxEvents)
	if err != nil {
		return Bundle{}, err
	}
	if len(records) == 0 {
		return Bundle{}, fmt.Errorf("incident %q contains no frozen events", incidentID)
	}

	allRuns, err := source.ListReplayRuns(ctx, 200)
	if err != nil {
		return Bundle{}, fmt.Errorf("read replay history: %w", err)
	}
	runs := make([]replay.Run, 0)
	for _, run := range allRuns {
		if run.IncidentID == incidentID {
			runs = append(runs, run)
		}
	}

	allCosts, err := source.ListCostSimulations(ctx, 200)
	if err != nil {
		return Bundle{}, fmt.Errorf("read cost simulation history: %w", err)
	}
	costs := make([]costsim.Result, 0)
	for _, result := range allCosts {
		if result.IncidentID == incidentID {
			costs = append(costs, result)
		}
	}

	pairs := incidentSourceTypes(records)
	schemas := make([]schema.RegistryEntry, 0)
	for _, pair := range pairs {
		history, err := source.SchemaHistory(ctx, pair.Source, pair.EventType)
		if err != nil {
			return Bundle{}, fmt.Errorf("read schema history for %s/%s: %w", pair.Source, pair.EventType, err)
		}
		schemas = append(schemas, history...)
	}

	allDrifts, err := source.ListSchemaDrifts(ctx, 500)
	if err != nil {
		return Bundle{}, fmt.Errorf("read schema drift history: %w", err)
	}
	drifts := make([]schema.Drift, 0)
	allowedPairs := make(map[string]struct{}, len(pairs))
	for _, pair := range pairs {
		allowedPairs[pair.Source+"\x00"+pair.EventType] = struct{}{}
	}
	for _, drift := range allDrifts {
		if _, ok := allowedPairs[drift.Source+"\x00"+drift.EventType]; ok {
			drifts = append(drifts, drift)
		}
	}

	events := make([]domain.Event, 0, len(records))
	tenantID := ""
	for _, record := range records {
		events = append(events, record.Event)
		if tenantID == "" {
			tenantID = record.Event.TenantID
		}
	}

	var changes []changeintel.Marker
	if changeReader, ok := source.(changeSource); ok && len(records) > 0 {
		from, to := records[0].Event.Timestamp, records[0].Event.Timestamp
		for _, record := range records[1:] {
			if record.Event.Timestamp.Before(from) {
				from = record.Event.Timestamp
			}
			if record.Event.Timestamp.After(to) {
				to = record.Event.Timestamp
			}
		}
		changes, _ = changeReader.ChangesBetween(ctx, from.Add(-15*time.Minute), to.Add(5*time.Minute), 100)
	}
	graph := evidence.BuildWithChanges(tenantID, incidentID, events, runs, costs, changes)

	return Bundle{
		Incident:       incident,
		Events:         records,
		EvidenceGraph:  graph,
		ReplayRuns:     runs,
		CostResults:    costs,
		Schemas:        schemas,
		SchemaDrifts:   drifts,
		Changes:        changes,
		Configurations: configurations,
	}, nil
}

// LoadConfigurationSnapshot preserves exact JSON bytes while recording common
// name/version metadata when present.
func LoadConfigurationSnapshot(kind, role, filename string) (ConfigurationSnapshot, error) {
	payload, err := os.ReadFile(filename)
	if err != nil {
		return ConfigurationSnapshot{}, fmt.Errorf("read %s %s configuration: %w", kind, role, err)
	}
	if !json.Valid(payload) {
		return ConfigurationSnapshot{}, fmt.Errorf("%s %s configuration is not valid JSON", kind, role)
	}

	var metadata struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	_ = json.Unmarshal(payload, &metadata)

	return ConfigurationSnapshot{
		Kind:    strings.TrimSpace(kind),
		Role:    strings.TrimSpace(role),
		Name:    strings.TrimSpace(metadata.Name),
		Version: strings.TrimSpace(metadata.Version),
		Payload: append(json.RawMessage(nil), payload...),
	}, nil
}

type sourceType struct {
	Source    string
	EventType string
}

func incidentSourceTypes(records []EventRecord) []sourceType {
	seen := make(map[string]sourceType)
	for _, record := range records {
		key := record.Event.Source + "\x00" + record.Event.Type
		seen[key] = sourceType{Source: record.Event.Source, EventType: record.Event.Type}
	}
	result := make([]sourceType, 0, len(seen))
	for _, pair := range seen {
		result = append(result, pair)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Source == result[j].Source {
			return result[i].EventType < result[j].EventType
		}
		return result[i].Source < result[j].Source
	})
	return result
}
