package connectors

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"io"
	"net/http"
)

type httpJSONConnector struct {
	spec   Spec
	client *http.Client
}

func newHTTPJSONConnector(spec Spec, f FactoryConfig) (Connector, error) {
	if err := requireSecretEnvironment(spec); err != nil {
		return nil, err
	}
	return &httpJSONConnector{spec: spec, client: clientFor(spec, f)}, nil
}
func (c *httpJSONConnector) Kind() string               { return KindHTTPJSON }
func (c *httpJSONConnector) Capabilities() Capabilities { return capability(KindHTTPJSON) }
func (c *httpJSONConnector) Send(ctx context.Context, e domain.Event) error {
	p, err := json.Marshal(e)
	if err != nil {
		return &DeliveryError{Err: fmt.Errorf("encode canonical event: %w", err), Retryable: false}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.spec.Endpoint, bytes.NewReader(p))
	if err != nil {
		return &DeliveryError{Err: err, Retryable: false}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", e.ID)
	req.Header.Set("X-TelemetryForge-Event-ID", e.ID)
	req.Header.Set("X-TelemetryForge-Tenant-ID", e.TenantID)
	if err := applyHTTPHeaders(req, c.spec); err != nil {
		return &DeliveryError{Err: err, Retryable: false}
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return &DeliveryError{Err: fmt.Errorf("HTTP connector request: %w", err), Retryable: true}
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64*1024))
	return classifyHTTPStatus(resp.StatusCode, resp.Status)
}
func (c *httpJSONConnector) Ready(ctx context.Context) error {
	return readinessGET(ctx, c.client, c.spec)
}
func (c *httpJSONConnector) Close() {}
