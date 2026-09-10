package connectors

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"io"
	"net/http"
	"os"
	"strings"
)

type splunkHECConnector struct {
	spec   Spec
	client *http.Client
}

func newSplunkHECConnector(spec Spec, f FactoryConfig) (Connector, error) {
	if err := requireSecretEnvironment(spec); err != nil {
		return nil, err
	}
	return &splunkHECConnector{spec: spec, client: clientFor(spec, f)}, nil
}
func (c *splunkHECConnector) Kind() string               { return KindSplunkHEC }
func (c *splunkHECConnector) Capabilities() Capabilities { return capability(KindSplunkHEC) }
func (c *splunkHECConnector) Close()                     {}
func (c *splunkHECConnector) Ready(ctx context.Context) error {
	if strings.TrimSpace(c.spec.HealthEndpoint) == "" {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.spec.HealthEndpoint, nil)
	if err != nil {
		return err
	}
	token := strings.TrimSpace(os.Getenv(c.spec.APIKeyEnv))
	if token == "" {
		return fmt.Errorf("Splunk HEC token environment %q is empty", c.spec.APIKeyEnv)
	}
	req.Header.Set("Authorization", "Splunk "+token)
	if err := applyHTTPHeaders(req, c.spec); err != nil {
		return err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 16*1024))
	return classifyHTTPStatus(resp.StatusCode, resp.Status)
}
func (c *splunkHECConnector) Send(ctx context.Context, e domain.Event) error {
	payload := map[string]any{"time": float64(e.Timestamp.UnixNano()) / 1e9, "host": e.Source, "source": "telemetryforge", "sourcetype": e.Type, "event": e, "fields": map[string]any{"tenant_id": e.TenantID, "event_id": e.ID}}
	data, err := json.Marshal(payload)
	if err != nil {
		return &DeliveryError{Err: err, Retryable: false}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.spec.Endpoint, bytes.NewReader(data))
	if err != nil {
		return &DeliveryError{Err: err, Retryable: false}
	}
	req.Header.Set("Content-Type", "application/json")
	token := strings.TrimSpace(os.Getenv(c.spec.APIKeyEnv))
	if token == "" {
		return &DeliveryError{Err: fmt.Errorf("Splunk HEC token environment %q is empty", c.spec.APIKeyEnv), Retryable: false}
	}
	req.Header.Set("Authorization", "Splunk "+token)
	if err := applyHTTPHeaders(req, c.spec); err != nil {
		return &DeliveryError{Err: err, Retryable: false}
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return &DeliveryError{Err: fmt.Errorf("Splunk HEC request: %w", err), Retryable: true}
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64*1024))
	return classifyHTTPStatus(resp.StatusCode, resp.Status)
}

type datadogLogsConnector struct {
	spec   Spec
	client *http.Client
}

func newDatadogLogsConnector(spec Spec, f FactoryConfig) (Connector, error) {
	if err := requireSecretEnvironment(spec); err != nil {
		return nil, err
	}
	return &datadogLogsConnector{spec: spec, client: clientFor(spec, f)}, nil
}
func (c *datadogLogsConnector) Kind() string               { return KindDatadogLogs }
func (c *datadogLogsConnector) Capabilities() Capabilities { return capability(KindDatadogLogs) }
func (c *datadogLogsConnector) Close()                     {}
func (c *datadogLogsConnector) Ready(ctx context.Context) error {
	if strings.TrimSpace(c.spec.HealthEndpoint) == "" {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.spec.HealthEndpoint, nil)
	if err != nil {
		return err
	}
	key := strings.TrimSpace(os.Getenv(c.spec.APIKeyEnv))
	if key == "" {
		return fmt.Errorf("Datadog API key environment %q is empty", c.spec.APIKeyEnv)
	}
	req.Header.Set("DD-API-KEY", key)
	if err := applyHTTPHeaders(req, c.spec); err != nil {
		return err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 16*1024))
	return classifyHTTPStatus(resp.StatusCode, resp.Status)
}
func (c *datadogLogsConnector) Send(ctx context.Context, e domain.Event) error {
	message := string(e.Payload)
	if strings.TrimSpace(message) == "" {
		message = e.Type
	}
	item := map[string]any{"ddsource": "telemetryforge", "service": e.Source, "status": eventSeverity(e), "message": message, "telemetryforge_event_id": e.ID, "telemetryforge_tenant_id": e.TenantID, "telemetryforge_type": e.Type, "tags": e.Tags}
	data, err := json.Marshal([]any{item})
	if err != nil {
		return &DeliveryError{Err: err, Retryable: false}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.spec.Endpoint, bytes.NewReader(data))
	if err != nil {
		return &DeliveryError{Err: err, Retryable: false}
	}
	req.Header.Set("Content-Type", "application/json")
	key := strings.TrimSpace(os.Getenv(c.spec.APIKeyEnv))
	if key == "" {
		return &DeliveryError{Err: fmt.Errorf("Datadog API key environment %q is empty", c.spec.APIKeyEnv), Retryable: false}
	}
	req.Header.Set("DD-API-KEY", key)
	if err := applyHTTPHeaders(req, c.spec); err != nil {
		return &DeliveryError{Err: err, Retryable: false}
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return &DeliveryError{Err: fmt.Errorf("Datadog Logs request: %w", err), Retryable: true}
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64*1024))
	return classifyHTTPStatus(resp.StatusCode, resp.Status)
}
func eventSeverity(e domain.Event) string {
	for _, k := range []string{"severity", "level"} {
		if v := strings.ToLower(strings.TrimSpace(e.Tags[k])); v != "" {
			return v
		}
	}
	if strings.Contains(strings.ToLower(e.Type), "error") {
		return "error"
	}
	return "info"
}
