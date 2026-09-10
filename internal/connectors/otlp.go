package connectors

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
)

const ProductVersion = "1.8.0"

type otlpHTTPConnector struct {
	spec   Spec
	client *http.Client
}

func newOTLPHTTPConnector(spec Spec, f FactoryConfig) (Connector, error) {
	if err := requireSecretEnvironment(spec); err != nil {
		return nil, err
	}
	return &otlpHTTPConnector{spec: spec, client: clientFor(spec, f)}, nil
}
func (c *otlpHTTPConnector) Kind() string               { return KindOTLPHTTP }
func (c *otlpHTTPConnector) Capabilities() Capabilities { return capability(KindOTLPHTTP) }
func (c *otlpHTTPConnector) Send(ctx context.Context, e domain.Event) error {
	endpoint := otlpSignalEndpoint(c.spec.Endpoint, "logs")
	var payload any = otlpLogsPayload(e)
	if e.Value != nil {
		endpoint = otlpSignalEndpoint(c.spec.Endpoint, "metrics")
		payload = otlpMetricsPayload(e)
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return &DeliveryError{Err: fmt.Errorf("encode OTLP JSON: %w", err), Retryable: false}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return &DeliveryError{Err: err, Retryable: false}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-TelemetryForge-Event-ID", e.ID)
	if err := applyHTTPHeaders(req, c.spec); err != nil {
		return &DeliveryError{Err: err, Retryable: false}
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return &DeliveryError{Err: fmt.Errorf("OTLP/HTTP request: %w", err), Retryable: true}
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64*1024))
	return classifyHTTPStatus(resp.StatusCode, resp.Status)
}
func (c *otlpHTTPConnector) Ready(ctx context.Context) error {
	return readinessGET(ctx, c.client, c.spec)
}
func (c *otlpHTTPConnector) Close() {}
func otlpSignalEndpoint(base, signal string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	p, err := url.Parse(base)
	if err == nil && strings.HasSuffix(p.Path, "/v1/logs") {
		if signal == "logs" {
			return base
		}
		p.Path = strings.TrimSuffix(p.Path, "/v1/logs") + "/v1/metrics"
		return p.String()
	}
	if err == nil && strings.HasSuffix(p.Path, "/v1/metrics") {
		if signal == "metrics" {
			return base
		}
		p.Path = strings.TrimSuffix(p.Path, "/v1/metrics") + "/v1/logs"
		return p.String()
	}
	return base + "/v1/" + signal
}
func otlpLogsPayload(e domain.Event) map[string]any {
	body := string(e.Payload)
	if strings.TrimSpace(body) == "" {
		body = e.Type
	}
	rec := map[string]any{"timeUnixNano": unixNanoString(e.Timestamp), "observedTimeUnixNano": unixNanoString(time.Now().UTC()), "body": map[string]any{"stringValue": body}, "attributes": otlpAttributes(e)}
	if sev := eventSeverity(e); sev != "" {
		rec["severityText"] = sev
	}
	scope := map[string]any{"scope": map[string]any{"name": "telemetryforge", "version": ProductVersion}, "logRecords": []any{rec}}
	if e.SchemaURL != "" {
		scope["schemaUrl"] = e.SchemaURL
	}
	return map[string]any{"resourceLogs": []any{map[string]any{"resource": map[string]any{"attributes": []any{otlpAttribute("service.name", e.Source), otlpAttribute("telemetryforge.tenant_id", e.TenantID)}}, "scopeLogs": []any{scope}}}}
}
func otlpMetricsPayload(e domain.Event) map[string]any {
	point := map[string]any{"timeUnixNano": unixNanoString(e.Timestamp), "asDouble": *e.Value, "attributes": otlpAttributes(e)}
	metric := map[string]any{"name": sanitizeMetricName(e.Type), "gauge": map[string]any{"dataPoints": []any{point}}}
	if e.Unit != "" {
		metric["unit"] = e.Unit
	}
	scope := map[string]any{"scope": map[string]any{"name": "telemetryforge", "version": ProductVersion}, "metrics": []any{metric}}
	if e.SchemaURL != "" {
		scope["schemaUrl"] = e.SchemaURL
	}
	return map[string]any{"resourceMetrics": []any{map[string]any{"resource": map[string]any{"attributes": []any{otlpAttribute("service.name", e.Source), otlpAttribute("telemetryforge.tenant_id", e.TenantID)}}, "scopeMetrics": []any{scope}}}}
}
func otlpAttributes(e domain.Event) []any {
	a := []any{otlpAttribute("telemetryforge.event_id", e.ID), otlpAttribute("telemetryforge.event_type", e.Type), otlpAttribute("telemetryforge.schema_version", e.SchemaVersion)}
	if e.CorrelationID != "" {
		a = append(a, otlpAttribute("telemetryforge.correlation_id", e.CorrelationID))
	}
	for k, v := range e.Tags {
		a = append(a, otlpAttribute(k, v))
	}
	return a
}
func otlpAttribute(k, v string) map[string]any {
	return map[string]any{"key": k, "value": map[string]any{"stringValue": v}}
}
func unixNanoString(t time.Time) string { return fmt.Sprintf("%d", t.UTC().UnixNano()) }
