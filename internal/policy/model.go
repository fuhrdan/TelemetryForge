// Package policy implements versioned telemetry policy and cardinality control.
package policy

import (
	"fmt"
	"path"
	"strings"
)

// Action is the behavior a policy requests for a dangerous dimension.
type Action string

const (
	ActionAllow      Action = "allow"
	ActionDropTag    Action = "drop_tag"
	ActionQuarantine Action = "quarantine"
)

// Policy is a versioned, reviewable telemetry-policy document.
//
// JSON is used deliberately in v0.8.0 so policy files can be parsed with the
// Go standard library and reviewed in ordinary pull requests without adding a
// runtime policy engine dependency.
type Policy struct {
	Name                   string   `json:"name"`
	Version                string   `json:"version"`
	DefaultUniqueThreshold uint64   `json:"default_unique_threshold"`
	DefaultAction          Action   `json:"default_action"`
	DangerousKeys          []string `json:"dangerous_keys"`
	Rules                  []Rule   `json:"rules,omitempty"`
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
	if !validAction(policy.DefaultAction) {
		return fmt.Errorf("invalid default action %q", policy.DefaultAction)
	}
	for index, rule := range policy.Rules {
		if strings.TrimSpace(rule.Dimension) == "" {
			return fmt.Errorf("rule %d dimension is required", index)
		}
		if rule.UniqueThreshold < 2 {
			return fmt.Errorf("rule %d unique_threshold must be at least 2", index)
		}
		if !validAction(rule.Action) {
			return fmt.Errorf("rule %d has invalid action %q", index, rule.Action)
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
