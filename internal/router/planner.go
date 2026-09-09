package router

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
)

// PlanRecorder is the durable boundary used by the production worker.
type PlanRecorder interface {
	EnqueueRoutingPlan(context.Context, domain.Event, []Decision) error
	RecordRoutingShadowDiff(context.Context, ShadowDiff) error
}

// Planner evaluates active routing and optional candidate shadow routing.
type Planner struct {
	active Config
	shadow *Config
	store  PlanRecorder
}

func NewPlanner(active Config, shadow *Config, store PlanRecorder) (*Planner, error) {
	if err := active.Validate(); err != nil {
		return nil, fmt.Errorf("active routing config: %w", err)
	}
	if shadow != nil {
		if err := shadow.Validate(); err != nil {
			return nil, fmt.Errorf("shadow routing config: %w", err)
		}
	}
	if store == nil {
		return nil, fmt.Errorf("routing plan recorder is required")
	}
	return &Planner{active: active, shadow: shadow, store: store}, nil
}

// Process records active delivery intents and non-destructive shadow diffs.
func (planner *Planner) Process(ctx context.Context, event domain.Event) error {
	active := planner.active.Plan(event)
	if err := planner.store.EnqueueRoutingPlan(ctx, event, active); err != nil {
		return fmt.Errorf("enqueue active routing plan: %w", err)
	}

	if planner.shadow == nil {
		return nil
	}
	shadow := planner.shadow.Plan(event)
	diff, changed := ComparePlans(event, planner.active, *planner.shadow, active, shadow, time.Now().UTC())
	if !changed {
		return nil
	}
	if err := planner.store.RecordRoutingShadowDiff(ctx, diff); err != nil {
		return fmt.Errorf("record shadow routing difference: %w", err)
	}
	return nil
}

// Plan returns a deduplicated fan-out plan for one event.
func (config Config) Plan(event domain.Event) []Decision {
	rulesByDestination := make(map[string][]string)
	matchedAny := false

	for _, rule := range config.Rules {
		if !ruleMatches(rule, event) {
			continue
		}
		matchedAny = true
		for _, destinationName := range rule.Destinations {
			destination, ok := config.DestinationByName(destinationName)
			if !ok || !destination.Enabled {
				continue
			}
			rulesByDestination[destinationName] = appendUnique(rulesByDestination[destinationName], rule.Name)
		}
		if rule.Stop {
			break
		}
	}

	if !matchedAny && config.FallbackDestination != "" {
		rulesByDestination[config.FallbackDestination] = []string{"fallback"}
	}

	names := make([]string, 0, len(rulesByDestination))
	for name := range rulesByDestination {
		names = append(names, name)
	}
	sort.Strings(names)

	result := make([]Decision, 0, len(names))
	for _, name := range names {
		destination, _ := config.DestinationByName(name)
		result = append(result, Decision{
			Destination:     name,
			RouteRules:      rulesByDestination[name],
			MaxAttempts:     destination.MaxAttempts,
			BaseDelayMS:     destination.BaseDelayMS,
			MaxDelayMS:      destination.MaxDelayMS,
			FailureFallback: destination.FailureFallback,
		})
	}
	return result
}

// ComparePlans returns only destination-set differences; shadow never enqueues.
func ComparePlans(event domain.Event, activeConfig, shadowConfig Config, active, shadow []Decision, observedAt time.Time) (ShadowDiff, bool) {
	activeNames := decisionNames(active)
	shadowNames := decisionNames(shadow)
	added := setDifference(shadowNames, activeNames)
	removed := setDifference(activeNames, shadowNames)
	if len(added) == 0 && len(removed) == 0 {
		return ShadowDiff{}, false
	}
	return ShadowDiff{
		TenantID: event.TenantID, EventID: event.ID, ObservedAt: observedAt,
		ActiveConfig: activeConfig.Name, ActiveVersion: activeConfig.Version,
		ShadowConfig: shadowConfig.Name, ShadowVersion: shadowConfig.Version,
		ActiveDestinations: activeNames, ShadowDestinations: shadowNames,
		Added: added, Removed: removed,
	}, true
}

func ruleMatches(rule Rule, event domain.Event) bool {
	if !glob(rule.Tenant, event.TenantID) || !glob(rule.Source, event.Source) || !glob(rule.Type, event.Type) {
		return false
	}
	if len(rule.Severity) > 0 {
		severity := eventSeverity(event)
		matched := false
		for _, candidate := range rule.Severity {
			if strings.EqualFold(strings.TrimSpace(candidate), severity) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	for key, pattern := range rule.Tags {
		value, ok := event.Tags[key]
		if !ok || !glob(pattern, value) {
			return false
		}
	}
	return true
}

func decisionNames(decisions []Decision) []string {
	names := make([]string, 0, len(decisions))
	for _, decision := range decisions {
		names = append(names, decision.Destination)
	}
	sort.Strings(names)
	return names
}

func setDifference(left, right []string) []string {
	rightSet := make(map[string]struct{}, len(right))
	for _, value := range right {
		rightSet[value] = struct{}{}
	}
	result := make([]string, 0)
	for _, value := range left {
		if _, exists := rightSet[value]; !exists {
			result = append(result, value)
		}
	}
	return result
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
