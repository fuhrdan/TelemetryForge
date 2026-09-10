// Package router implements TelemetryForge multi-destination routing policy,
// durable delivery planning, and isolated destination dispatch.
package router

import (
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/connectors"
	"github.com/fuhrdan/TelemetryForge/internal/domain"
)

const (
	DestinationKafka     = "kafka"
	DestinationHTTP      = "http"
	DestinationConnector = "connector"
)

// Config is a versioned routing policy and destination catalog.
type Config struct {
	Name                string        `json:"name"`
	Version             string        `json:"version"`
	Destinations        []Destination `json:"destinations"`
	Rules               []Rule        `json:"rules,omitempty"`
	FallbackDestination string        `json:"fallback_destination,omitempty"`
}

// Destination describes one independently delivered backend.
type Destination struct {
	Name            string            `json:"name"`
	Type            string            `json:"type"`
	Enabled         bool              `json:"enabled"`
	Brokers         []string          `json:"brokers,omitempty"`
	Topic           string            `json:"topic,omitempty"`
	URL             string            `json:"url,omitempty"`
	HealthURL       string            `json:"health_url,omitempty"`
	Headers         map[string]string `json:"headers,omitempty"`
	HeaderEnv       map[string]string `json:"header_env,omitempty"`
	BearerTokenEnv  string            `json:"bearer_token_env,omitempty"`
	Connector       *connectors.Spec  `json:"connector,omitempty"`
	TimeoutMS       int               `json:"timeout_ms,omitempty"`
	MaxAttempts     int               `json:"max_attempts,omitempty"`
	BaseDelayMS     int               `json:"base_delay_ms,omitempty"`
	MaxDelayMS      int               `json:"max_delay_ms,omitempty"`
	Concurrency     int               `json:"concurrency,omitempty"`
	FailureFallback string            `json:"failure_fallback,omitempty"`
}

// Rule fans matching telemetry out to one or more destinations.
// Empty Tenant/Source/Type match everything. Tag patterns use path.Match.
type Rule struct {
	Name         string            `json:"name"`
	Tenant       string            `json:"tenant,omitempty"`
	Source       string            `json:"source,omitempty"`
	Type         string            `json:"type,omitempty"`
	Severity     []string          `json:"severity,omitempty"`
	Tags         map[string]string `json:"tags,omitempty"`
	Destinations []string          `json:"destinations"`
	Stop         bool              `json:"stop,omitempty"`
}

// Decision is one durable event/destination delivery intent.
type Decision struct {
	Destination     string   `json:"destination"`
	RouteRules      []string `json:"route_rules,omitempty"`
	MaxAttempts     int      `json:"max_attempts"`
	BaseDelayMS     int      `json:"base_delay_ms"`
	MaxDelayMS      int      `json:"max_delay_ms"`
	FailureFallback string   `json:"failure_fallback,omitempty"`
}

// ShadowDiff records how candidate routing differs from active routing.
type ShadowDiff struct {
	TenantID           string    `json:"tenant_id,omitempty"`
	EventID            string    `json:"event_id"`
	ObservedAt         time.Time `json:"observed_at"`
	ActiveConfig       string    `json:"active_config"`
	ActiveVersion      string    `json:"active_version"`
	ShadowConfig       string    `json:"shadow_config"`
	ShadowVersion      string    `json:"shadow_version"`
	ActiveDestinations []string  `json:"active_destinations"`
	ShadowDestinations []string  `json:"shadow_destinations"`
	Added              []string  `json:"added"`
	Removed            []string  `json:"removed"`
}

// Delivery is one durable outbox row claimed by the router service.
type Delivery struct {
	TenantID        string       `json:"tenant_id"`
	EventID         string       `json:"event_id"`
	Destination     string       `json:"destination"`
	RouteRules      []string     `json:"route_rules,omitempty"`
	Event           domain.Event `json:"event"`
	Status          string       `json:"status"`
	Attempts        int          `json:"attempts"`
	MaxAttempts     int          `json:"max_attempts"`
	BaseDelayMS     int          `json:"base_delay_ms"`
	MaxDelayMS      int          `json:"max_delay_ms"`
	FailureFallback string       `json:"failure_fallback,omitempty"`
	NextAttemptAt   time.Time    `json:"next_attempt_at"`
	LeaseUntil      *time.Time   `json:"lease_until,omitempty"`
	LastError       string       `json:"last_error,omitempty"`
	CreatedAt       time.Time    `json:"created_at"`
	UpdatedAt       time.Time    `json:"updated_at"`
	DeliveredAt     *time.Time   `json:"delivered_at,omitempty"`
}

// DeadLetter is a terminal destination delivery failure.
type DeadLetter struct {
	TenantID    string       `json:"tenant_id"`
	EventID     string       `json:"event_id"`
	Destination string       `json:"destination"`
	RouteRules  []string     `json:"route_rules,omitempty"`
	Event       domain.Event `json:"event"`
	Attempts    int          `json:"attempts"`
	Error       string       `json:"error"`
	FailedAt    time.Time    `json:"failed_at"`
}

// DestinationHealth summarizes tenant-visible delivery health.
type DestinationHealth struct {
	TenantID            string     `json:"tenant_id,omitempty"`
	Destination         string     `json:"destination"`
	State               string     `json:"state"`
	ConsecutiveFailures int        `json:"consecutive_failures"`
	LastSuccessAt       *time.Time `json:"last_success_at,omitempty"`
	LastFailureAt       *time.Time `json:"last_failure_at,omitempty"`
	LastError           string     `json:"last_error,omitempty"`
	UpdatedAt           time.Time  `json:"updated_at"`
	Pending             int64      `json:"pending"`
	Retrying            int64      `json:"retrying"`
	DeadLetters         int64      `json:"dead_letters"`
}

// Validate rejects ambiguous/unsafe route documents before worker/router startup.
func (config Config) Validate() error {
	if strings.TrimSpace(config.Name) == "" {
		return fmt.Errorf("routing config name is required")
	}
	if strings.TrimSpace(config.Version) == "" {
		return fmt.Errorf("routing config version is required")
	}
	if len(config.Destinations) == 0 {
		return fmt.Errorf("at least one routing destination is required")
	}
	if len(config.Destinations) > 64 {
		return fmt.Errorf("routing config has %d destinations; maximum is 64", len(config.Destinations))
	}
	if len(config.Rules) > 512 {
		return fmt.Errorf("routing config has %d rules; maximum is 512", len(config.Rules))
	}

	destinations := make(map[string]Destination, len(config.Destinations))
	for index, destination := range config.Destinations {
		destination = withDestinationDefaults(destination)
		name := strings.TrimSpace(destination.Name)
		if name == "" {
			return fmt.Errorf("destination %d name is required", index)
		}
		if _, exists := destinations[name]; exists {
			return fmt.Errorf("duplicate destination %q", name)
		}
		if destination.Type != DestinationKafka &&
			destination.Type != DestinationHTTP &&
			destination.Type != DestinationConnector {
			return fmt.Errorf("destination %q has unsupported type %q", name, destination.Type)
		}
		switch destination.Type {
		case DestinationKafka:
			if destination.Connector != nil {
				return fmt.Errorf("legacy Kafka destination %q cannot also define connector", name)
			}
			if strings.TrimSpace(destination.Topic) == "" {
				return fmt.Errorf("Kafka destination %q requires topic", name)
			}
		case DestinationHTTP:
			if destination.Connector != nil {
				return fmt.Errorf("legacy HTTP destination %q cannot also define connector", name)
			}
			if strings.TrimSpace(destination.URL) == "" {
				return fmt.Errorf("HTTP destination %q requires url", name)
			}
			if !strings.HasPrefix(destination.URL, "http://") && !strings.HasPrefix(destination.URL, "https://") {
				return fmt.Errorf("HTTP destination %q URL must use http or https", name)
			}
			for headerName := range destination.Headers {
				if strings.TrimSpace(headerName) == "" {
					return fmt.Errorf("HTTP destination %q has empty static header name", name)
				}
				if sensitiveHeaderName(headerName) {
					return fmt.Errorf("HTTP destination %q cannot store sensitive header %q directly; use header_env or bearer_token_env", name, headerName)
				}
			}
			for headerName, environmentName := range destination.HeaderEnv {
				if strings.TrimSpace(headerName) == "" || strings.TrimSpace(environmentName) == "" {
					return fmt.Errorf("HTTP destination %q header_env requires non-empty header and environment names", name)
				}
			}
		case DestinationConnector:
			if destination.Connector == nil {
				return fmt.Errorf("connector destination %q requires connector configuration", name)
			}
			if err := destination.Connector.Validate(); err != nil {
				return fmt.Errorf("connector destination %q: %w", name, err)
			}
			for _, legacy := range []struct {
				label string
				set   bool
			}{
				{"brokers", len(destination.Brokers) > 0}, {"topic", strings.TrimSpace(destination.Topic) != ""},
				{"url", strings.TrimSpace(destination.URL) != ""}, {"health_url", strings.TrimSpace(destination.HealthURL) != ""},
				{"headers", len(destination.Headers) > 0}, {"header_env", len(destination.HeaderEnv) > 0},
				{"bearer_token_env", strings.TrimSpace(destination.BearerTokenEnv) != ""},
			} {
				if legacy.set {
					return fmt.Errorf("connector destination %q must put %s inside connector", name, legacy.label)
				}
			}
		}
		destinations[name] = destination
	}

	if fallback := strings.TrimSpace(config.FallbackDestination); fallback != "" {
		if destination, ok := destinations[fallback]; !ok || !destination.Enabled {
			return fmt.Errorf("fallback destination %q is missing or disabled", fallback)
		}
	}

	seenRules := make(map[string]struct{}, len(config.Rules))
	for index, rule := range config.Rules {
		name := strings.TrimSpace(rule.Name)
		if name == "" {
			return fmt.Errorf("routing rule %d name is required", index)
		}
		if _, exists := seenRules[name]; exists {
			return fmt.Errorf("duplicate routing rule %q", name)
		}
		seenRules[name] = struct{}{}
		for label, pattern := range map[string]string{"tenant": rule.Tenant, "source": rule.Source, "type": rule.Type} {
			if err := validateGlob("routing rule "+label, pattern); err != nil {
				return fmt.Errorf("routing rule %q: %w", name, err)
			}
		}
		for key, pattern := range rule.Tags {
			if strings.TrimSpace(key) == "" {
				return fmt.Errorf("routing rule %q has empty tag key", name)
			}
			if err := validateGlob("tag "+key, pattern); err != nil {
				return fmt.Errorf("routing rule %q: %w", name, err)
			}
		}
		if len(rule.Destinations) == 0 {
			return fmt.Errorf("routing rule %q requires at least one destination", name)
		}
		for _, destinationName := range rule.Destinations {
			destination, ok := destinations[destinationName]
			if !ok {
				return fmt.Errorf("routing rule %q references unknown destination %q", name, destinationName)
			}
			if !destination.Enabled {
				return fmt.Errorf("routing rule %q references disabled destination %q", name, destinationName)
			}
		}
	}

	for name, destination := range destinations {
		if destination.FailureFallback == "" {
			continue
		}
		fallback, ok := destinations[destination.FailureFallback]
		if !ok || !fallback.Enabled {
			return fmt.Errorf("destination %q failure_fallback %q is missing or disabled", name, destination.FailureFallback)
		}
		if destination.FailureFallback == name {
			return fmt.Errorf("destination %q cannot fall back to itself", name)
		}
	}
	if err := validateFallbackCycles(destinations); err != nil {
		return err
	}
	return nil
}

// DestinationByName returns one destination with operational defaults applied.
func (config Config) DestinationByName(name string) (Destination, bool) {
	for _, destination := range config.Destinations {
		if destination.Name == name {
			return withDestinationDefaults(destination), true
		}
	}
	return Destination{}, false
}

// EnabledDestinations returns stable names for dashboard/CLI display.
func (config Config) EnabledDestinations() []Destination {
	result := make([]Destination, 0, len(config.Destinations))
	for _, destination := range config.Destinations {
		if destination.Enabled {
			result = append(result, withDestinationDefaults(destination))
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func withDestinationDefaults(destination Destination) Destination {
	if destination.TimeoutMS <= 0 {
		destination.TimeoutMS = 5000
	}
	if destination.MaxAttempts <= 0 {
		destination.MaxAttempts = 5
	}
	if destination.BaseDelayMS <= 0 {
		destination.BaseDelayMS = 500
	}
	if destination.MaxDelayMS <= 0 {
		destination.MaxDelayMS = 30000
	}
	if destination.Concurrency <= 0 {
		destination.Concurrency = 2
	}
	if destination.MaxDelayMS < destination.BaseDelayMS {
		destination.MaxDelayMS = destination.BaseDelayMS
	}
	return destination
}

// ConnectorSpec returns the normalized connector definition for one destination.
func ConnectorSpec(destination Destination) (connectors.Spec, error) {
	destination = withDestinationDefaults(destination)
	switch destination.Type {
	case DestinationConnector:
		if destination.Connector == nil {
			return connectors.Spec{}, fmt.Errorf("destination %q has no connector configuration", destination.Name)
		}
		spec := *destination.Connector
		if spec.TimeoutMS <= 0 {
			spec.TimeoutMS = destination.TimeoutMS
		}
		spec = connectors.WithDefaults(spec)
		return spec, spec.Validate()
	case DestinationKafka:
		spec := connectors.Spec{Kind: connectors.KindKafka, Brokers: destination.Brokers, Topic: destination.Topic, TimeoutMS: destination.TimeoutMS}
		return connectors.WithDefaults(spec), spec.Validate()
	case DestinationHTTP:
		spec := connectors.Spec{Kind: connectors.KindHTTPJSON, Endpoint: destination.URL, HealthEndpoint: destination.HealthURL, Headers: destination.Headers, HeaderEnv: destination.HeaderEnv, BearerTokenEnv: destination.BearerTokenEnv, TimeoutMS: destination.TimeoutMS}
		return connectors.WithDefaults(spec), spec.Validate()
	default:
		return connectors.Spec{}, fmt.Errorf("destination %q has unsupported type %q", destination.Name, destination.Type)
	}
}

func validateFallbackCycles(destinations map[string]Destination) error {
	for start := range destinations {
		seen := map[string]struct{}{}
		current := start
		for current != "" {
			if _, exists := seen[current]; exists {
				return fmt.Errorf("routing failure fallback cycle includes %q", current)
			}
			seen[current] = struct{}{}
			next := destinations[current].FailureFallback
			current = next
		}
	}
	return nil
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

func eventSeverity(event domain.Event) string {
	for _, key := range []string{"severity", "level"} {
		if value := strings.ToLower(strings.TrimSpace(event.Tags[key])); value != "" {
			return value
		}
	}
	if strings.Contains(strings.ToLower(event.Type), "error") {
		return "error"
	}
	return ""
}

func sensitiveHeaderName(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	for _, marker := range []string{"authorization", "cookie", "token", "api-key", "apikey", "secret"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
