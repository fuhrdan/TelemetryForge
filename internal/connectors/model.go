// Package connectors defines the stable TelemetryForge connector contract.
//
// Connectors translate the canonical post-policy event into an external
// backend protocol. They never own durable retry state; the Telemetry Router
// outbox/dispatcher remains the failure-isolation and retry boundary.
package connectors

import (
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	KindKafka                 = "kafka"
	KindHTTPJSON              = "http_json"
	KindOTLPHTTP              = "otlp_http"
	KindPrometheusRemoteWrite = "prometheus_remote_write"
	KindSplunkHEC             = "splunk_hec"
	KindDatadogLogs           = "datadog_logs"
)

type Spec struct {
	Kind             string            `json:"kind"`
	Endpoint         string            `json:"endpoint,omitempty"`
	HealthEndpoint   string            `json:"health_endpoint,omitempty"`
	Brokers          []string          `json:"brokers,omitempty"`
	Topic            string            `json:"topic,omitempty"`
	Headers          map[string]string `json:"headers,omitempty"`
	HeaderEnv        map[string]string `json:"header_env,omitempty"`
	BearerTokenEnv   string            `json:"bearer_token_env,omitempty"`
	APIKeyEnv        string            `json:"api_key_env,omitempty"`
	TimeoutMS        int               `json:"timeout_ms,omitempty"`
	PrometheusLabels []string          `json:"prometheus_label_tags,omitempty"`
}

type Capabilities struct {
	Kind                string   `json:"kind"`
	Protocol            string   `json:"protocol"`
	Signals             []string `json:"signals"`
	HealthCheck         bool     `json:"health_check"`
	RetryClassification bool     `json:"retry_classification"`
	Notes               string   `json:"notes,omitempty"`
}

type RuntimeState struct {
	InstanceID   string       `json:"instance_id"`
	Destination  string       `json:"destination"`
	Kind         string       `json:"kind"`
	Capabilities Capabilities `json:"capabilities"`
	Ready        bool         `json:"ready"`
	LastError    string       `json:"last_error,omitempty"`
	CheckedAt    time.Time    `json:"checked_at"`
}

func Catalog() []Capabilities {
	result := []Capabilities{
		capability(KindKafka), capability(KindHTTPJSON), capability(KindOTLPHTTP),
		capability(KindPrometheusRemoteWrite), capability(KindSplunkHEC), capability(KindDatadogLogs),
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Kind < result[j].Kind })
	return result
}

func Capability(kind string) (Capabilities, bool) {
	kind = normalizeKind(kind)
	for _, candidate := range Catalog() {
		if candidate.Kind == kind {
			return candidate, true
		}
	}
	return Capabilities{}, false
}

func capability(kind string) Capabilities {
	switch normalizeKind(kind) {
	case KindKafka:
		return Capabilities{Kind: KindKafka, Protocol: "Kafka", Signals: []string{"events", "metrics", "logs", "changes"}, HealthCheck: true, RetryClassification: true, Notes: "Publishes the canonical TelemetryForge envelope."}
	case KindHTTPJSON:
		return Capabilities{Kind: KindHTTPJSON, Protocol: "HTTP JSON", Signals: []string{"events", "metrics", "logs", "changes"}, HealthCheck: true, RetryClassification: true, Notes: "Posts the canonical TelemetryForge envelope."}
	case KindOTLPHTTP:
		return Capabilities{Kind: KindOTLPHTTP, Protocol: "OTLP/HTTP JSON", Signals: []string{"logs", "metrics"}, HealthCheck: true, RetryClassification: true, Notes: "Metric envelopes become OTLP gauges; non-metrics become OTLP log records."}
	case KindPrometheusRemoteWrite:
		return Capabilities{Kind: KindPrometheusRemoteWrite, Protocol: "Prometheus Remote Write 0.1", Signals: []string{"metrics"}, HealthCheck: true, RetryClassification: true, Notes: "Exports numeric canonical events as one-sample time series."}
	case KindSplunkHEC:
		return Capabilities{Kind: KindSplunkHEC, Protocol: "Splunk HTTP Event Collector", Signals: []string{"events", "metrics", "logs", "changes"}, HealthCheck: true, RetryClassification: true}
	case KindDatadogLogs:
		return Capabilities{Kind: KindDatadogLogs, Protocol: "Datadog Logs HTTP intake", Signals: []string{"logs", "events", "changes"}, HealthCheck: true, RetryClassification: true}
	}
	return Capabilities{}
}

func (spec Spec) Validate() error {
	spec = withDefaults(spec)
	kind := normalizeKind(spec.Kind)
	if _, ok := Capability(kind); !ok {
		return fmt.Errorf("unsupported connector kind %q", spec.Kind)
	}
	if len(spec.Headers) > 32 || len(spec.HeaderEnv) > 32 {
		return errors.New("connector supports at most 32 static headers and 32 environment-backed headers")
	}
	if len(spec.PrometheusLabels) > 32 {
		return errors.New("prometheus_label_tags supports at most 32 tag names")
	}
	for name, value := range spec.Headers {
		if strings.TrimSpace(name) == "" {
			return errors.New("connector static header name cannot be empty")
		}
		if sensitiveHeaderName(name) {
			return fmt.Errorf("connector cannot store sensitive header %q directly; use header_env, bearer_token_env, or api_key_env", name)
		}
		if strings.ContainsAny(name, "\r\n") || strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("connector static header %q contains newline characters", name)
		}
	}
	for name, envName := range spec.HeaderEnv {
		if strings.TrimSpace(name) == "" || strings.TrimSpace(envName) == "" {
			return errors.New("connector header_env requires non-empty header/environment names")
		}
		if strings.ContainsAny(name, "\r\n") {
			return fmt.Errorf("connector header %q contains newline characters", name)
		}
	}
	switch kind {
	case KindKafka:
		if strings.TrimSpace(spec.Topic) == "" {
			return errors.New("Kafka connector requires topic")
		}
	case KindHTTPJSON, KindOTLPHTTP, KindPrometheusRemoteWrite, KindSplunkHEC, KindDatadogLogs:
		if err := validateHTTPEndpoint(spec.Endpoint); err != nil {
			return fmt.Errorf("%s connector: %w", kind, err)
		}
		if strings.TrimSpace(spec.HealthEndpoint) != "" {
			if err := validateHTTPEndpoint(spec.HealthEndpoint); err != nil {
				return fmt.Errorf("%s connector health endpoint: %w", kind, err)
			}
		}
	}
	if kind == KindSplunkHEC && strings.TrimSpace(spec.APIKeyEnv) == "" {
		return errors.New("Splunk HEC connector requires api_key_env containing the HEC token")
	}
	if kind == KindDatadogLogs && strings.TrimSpace(spec.APIKeyEnv) == "" {
		return errors.New("Datadog Logs connector requires api_key_env")
	}
	for _, label := range spec.PrometheusLabels {
		if strings.TrimSpace(label) == "" {
			return errors.New("prometheus_label_tags cannot contain empty names")
		}
	}
	return nil
}

func WithDefaults(spec Spec) Spec { return withDefaults(spec) }
func withDefaults(spec Spec) Spec {
	spec.Kind = normalizeKind(spec.Kind)
	if spec.TimeoutMS <= 0 {
		spec.TimeoutMS = 5000
	}
	return spec
}
func normalizeKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "http":
		return KindHTTPJSON
	case "otlp", "otlp-http":
		return KindOTLPHTTP
	case "prometheus", "remote_write", "prometheus-remote-write":
		return KindPrometheusRemoteWrite
	case "splunk":
		return KindSplunkHEC
	case "datadog":
		return KindDatadogLogs
	default:
		return strings.ToLower(strings.TrimSpace(kind))
	}
}
func validateHTTPEndpoint(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return errors.New("endpoint is required")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid endpoint: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("endpoint must use http or https")
	}
	if parsed.Host == "" {
		return errors.New("endpoint host is required")
	}
	if parsed.User != nil {
		return errors.New("endpoint must not embed credentials")
	}
	if parsed.Fragment != "" {
		return errors.New("endpoint must not contain a URL fragment")
	}
	for key := range parsed.Query() {
		if sensitiveHeaderName(key) || strings.Contains(strings.ToLower(key), "key") {
			return fmt.Errorf("endpoint query parameter %q looks credential-bearing; use environment-backed headers instead", key)
		}
	}
	return nil
}
func sensitiveHeaderName(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	for _, marker := range []string{"authorization", "cookie", "token", "api-key", "apikey", "secret", "password"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
