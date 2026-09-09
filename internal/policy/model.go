// Package policy implements versioned telemetry policy and cardinality control.
package policy

import (
	"fmt"
	"path"
	"strings"
	"time"
)

// Action is the behavior a policy requests for a dangerous dimension.
type Action string

const (
	maxPersistedCardinality uint64 = 1<<63 - 1

	ActionAllow      Action = "allow"
	ActionDropTag    Action = "drop_tag"
	ActionQuarantine Action = "quarantine"
)

// Policy is a versioned, reviewable telemetry-policy document.
//
// JSON is used deliberately so policy files can be parsed with the Go standard
// library and reviewed in ordinary pull requests without adding a runtime
// policy engine dependency.
type Policy struct {
	Name                   string   `json:"name"`
	Version                string   `json:"version"`
	DefaultUniqueThreshold uint64   `json:"default_unique_threshold"`
	DefaultAction          Action   `json:"default_action"`
	DangerousKeys          []string `json:"dangerous_keys"`
	Rules                  []Rule   `json:"rules,omitempty"`
	Budgets                []Budget `json:"budgets,omitempty"`
}

// Budget defines an hourly unique-series budget for matching telemetry.
// Budgets are visibility/alerting controls; they do not mutate telemetry.
type Budget struct {
	Name            string  `json:"name"`
	Source          string  `json:"source"`
	Type            string  `json:"type"`
	SeriesLimit     uint64  `json:"series_limit"`
	WarningPercent  float64 `json:"warning_percent"`
	CriticalPercent float64 `json:"critical_percent"`
}

// BudgetStatus is the latest observed cluster-wide consumption for one budget.
type BudgetStatus struct {
	TenantID           string    `json:"tenant_id,omitempty"`
	PolicyName         string    `json:"policy_name"`
	PolicyVersion      string    `json:"policy_version"`
	Mode               string    `json:"mode"`
	BudgetName         string    `json:"budget_name"`
	Source             string    `json:"source"`
	EventType          string    `json:"event_type"`
	WindowStart        time.Time `json:"window_start"`
	SeriesLimit        uint64    `json:"series_limit"`
	ObservedUnique     uint64    `json:"observed_unique"`
	ProjectedUnique    uint64    `json:"projected_unique"`
	ConsumptionPercent float64   `json:"consumption_percent"`
	Status             string    `json:"status"`
	ObservedAt         time.Time `json:"observed_at"`
}

// Rule overrides the default threshold/action for a matching dimension.
// Source, Type, and Dimension use path.Match-style glob patterns.
type Rule struct {
	Source          string `json:"source"`
	Type            string `json:"type"`
	Dimension       string `json:"dimension"`
	UniqueThreshold uint64 `json:"unique_threshold"`
	Action          Action `json:"action"`
}

// Validate rejects ambiguous or unsafe policy documents before worker startup.
func (policy Policy) Validate() error {
	if strings.TrimSpace(policy.Name) == "" {
		return fmt.Errorf("policy name is required")
	}
	if strings.TrimSpace(policy.Version) == "" {
		return fmt.Errorf("policy version is required")
	}
	if policy.DefaultUniqueThreshold < 2 {
		return fmt.Errorf("default_unique_threshold must be at least 2")
	}
	if policy.DefaultUniqueThreshold > maxPersistedCardinality {
		return fmt.Errorf("default_unique_threshold exceeds the supported signed 64-bit storage range")
	}
	if !validAction(policy.DefaultAction) {
		return fmt.Errorf("invalid default action %q", policy.DefaultAction)
	}
	for index, rule := range policy.Rules {
		if err := validateGlob("rule source", rule.Source); err != nil {
			return fmt.Errorf("rule %d: %w", index, err)
		}
		if err := validateGlob("rule type", rule.Type); err != nil {
			return fmt.Errorf("rule %d: %w", index, err)
		}
		if strings.TrimSpace(rule.Dimension) == "" {
			return fmt.Errorf("rule %d dimension is required", index)
		}
		if err := validateGlob("rule dimension", rule.Dimension); err != nil {
			return fmt.Errorf("rule %d: %w", index, err)
		}
		if rule.UniqueThreshold < 2 {
			return fmt.Errorf("rule %d unique_threshold must be at least 2", index)
		}
		if rule.UniqueThreshold > maxPersistedCardinality {
			return fmt.Errorf("rule %d unique_threshold exceeds the supported signed 64-bit storage range", index)
		}
		if !validAction(rule.Action) {
			return fmt.Errorf("rule %d has invalid action %q", index, rule.Action)
		}
	}
	seenBudgets := make(map[string]struct{}, len(policy.Budgets))
	for index, budget := range policy.Budgets {
		name := strings.TrimSpace(budget.Name)
		if name == "" {
			return fmt.Errorf("budget %d name is required", index)
		}
		if _, exists := seenBudgets[name]; exists {
			return fmt.Errorf("duplicate budget name %q", name)
		}
		seenBudgets[name] = struct{}{}
		if err := validateGlob("budget source", budget.Source); err != nil {
			return fmt.Errorf("budget %q: %w", name, err)
		}
		if err := validateGlob("budget type", budget.Type); err != nil {
			return fmt.Errorf("budget %q: %w", name, err)
		}
		if budget.SeriesLimit < 2 {
			return fmt.Errorf("budget %q series_limit must be at least 2", name)
		}
		if budget.SeriesLimit > maxPersistedCardinality {
			return fmt.Errorf("budget %q series_limit exceeds the supported signed 64-bit storage range", name)
		}
		if budget.WarningPercent <= 0 || budget.WarningPercent >= 100 {
			return fmt.Errorf("budget %q warning_percent must be between 0 and 100", name)
		}
		if budget.CriticalPercent <= budget.WarningPercent || budget.CriticalPercent > 100 {
			return fmt.Errorf("budget %q critical_percent must be greater than warning_percent and <= 100", name)
		}
	}
	return nil
}

// Decision returns the effective threshold/action and whether the key itself is
// identifier-shaped enough to deserve early scrutiny.
func (policy Policy) Decision(source, eventType, dimension string) (uint64, Action, bool) {
	for _, rule := range policy.Rules {
		if glob(rule.Source, source) && glob(rule.Type, eventType) && glob(rule.Dimension, dimension) {
			return rule.UniqueThreshold, rule.Action, policy.isDangerousKey(dimension)
		}
	}
	return policy.DefaultUniqueThreshold, policy.DefaultAction, policy.isDangerousKey(dimension)
}

// MatchingBudgets returns hourly series budgets that apply to a source/type.
func (policy Policy) MatchingBudgets(source, eventType string) []Budget {
	result := make([]Budget, 0, len(policy.Budgets))
	for _, budget := range policy.Budgets {
		if glob(budget.Source, source) && glob(budget.Type, eventType) {
			result = append(result, budget)
		}
	}
	return result
}

func (policy Policy) isDangerousKey(dimension string) bool {
	lower := strings.ToLower(dimension)
	for _, key := range policy.DangerousKeys {
		key = strings.ToLower(strings.TrimSpace(key))
		if key != "" && (lower == key || strings.Contains(lower, key)) {
			return true
		}
	}
	return false
}

func validateGlob(label, pattern string) error {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" || pattern == "*" {
		return nil
	}
	if _, err := path.Match(pattern, "validation-probe"); err != nil {
		return fmt.Errorf("%s pattern %q is invalid: %w", label, pattern, err)
	}
	return nil
}

func glob(pattern, value string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" || pattern == "*" {
		return true
	}
	matched, err := path.Match(pattern, value)
	return err == nil && matched
}

func validAction(action Action) bool {
	switch action {
	case ActionAllow, ActionDropTag, ActionQuarantine:
		return true
	default:
		return false
	}
}
