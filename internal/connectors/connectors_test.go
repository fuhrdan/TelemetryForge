package connectors

import (
	"context"
	"encoding/json"
	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testEvent() domain.Event {
	v := 42.5
	return domain.Event{ID: "evt-1", TenantID: "acme", Source: "checkout-api", Type: "request.duration", Timestamp: time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC), SchemaVersion: "1.0", CorrelationID: "corr-1", Tags: map[string]string{"environment": "production", "region": "us-west"}, Value: &v, Unit: "ms"}
}
func TestCatalogContainsBuiltInConnectorKinds(t *testing.T) {
	wanted := map[string]bool{KindKafka: true, KindHTTPJSON: true, KindOTLPHTTP: true, KindPrometheusRemoteWrite: true, KindSplunkHEC: true, KindDatadogLogs: true}
	for _, i := range Catalog() {
		delete(wanted, i.Kind)
	}
	if len(wanted) != 0 {
		t.Fatalf("missing: %#v", wanted)
	}
}
func TestSpecRejectsEmbeddedCredentialsAndStaticSecrets(t *testing.T) {
	for _, s := range []Spec{{Kind: KindHTTPJSON, Endpoint: "https://user:secret@example.invalid/hook"}, {Kind: KindHTTPJSON, Endpoint: "https://example.invalid/hook", Headers: map[string]string{"Authorization": "secret"}}, {Kind: KindDatadogLogs, Endpoint: "https://example.invalid/api/v2/logs"}} {
		if err := s.Validate(); err == nil {
			t.Fatalf("expected failure %#v", s)
		}
	}
}
func TestHTTPJSONClassifiesPermanentAndRetryableStatus(t *testing.T) {
	status := http.StatusBadRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Idempotency-Key") != "evt-1" {
			t.Errorf("missing idempotency")
		}
		w.WriteHeader(status)
	}))
	defer srv.Close()
	c, err := Build(Spec{Kind: KindHTTPJSON, Endpoint: srv.URL}, FactoryConfig{})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	err = c.Send(context.Background(), testEvent())
	if err == nil || Retryable(err) {
		t.Fatalf("400 permanent: %v", err)
	}
	status = http.StatusServiceUnavailable
	err = c.Send(context.Background(), testEvent())
	if err == nil || !Retryable(err) {
		t.Fatalf("503 retryable: %v", err)
	}
}
func TestOTLPHTTPSelectsMetricAndLogSignalEndpoints(t *testing.T) {
	paths := []string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		var p map[string]any
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			t.Errorf("json %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()
	c, err := Build(Spec{Kind: KindOTLPHTTP, Endpoint: srv.URL}, FactoryConfig{})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	m := testEvent()
	if err := c.Send(context.Background(), m); err != nil {
		t.Fatal(err)
	}
	l := testEvent()
	l.Value = nil
	l.Type = "request.error"
	l.Payload = json.RawMessage(`{"message":"boom"}`)
	if err := c.Send(context.Background(), l); err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 || paths[0] != "/v1/metrics" || paths[1] != "/v1/logs" {
		t.Fatalf("paths=%v", paths)
	}
}
func TestPrometheusRemoteWriteHeadersAndLabelAllowlist(t *testing.T) {
	var body []byte
	var ct, enc, ver string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ct = r.Header.Get("Content-Type")
		enc = r.Header.Get("Content-Encoding")
		ver = r.Header.Get("X-Prometheus-Remote-Write-Version")
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	c, err := Build(Spec{Kind: KindPrometheusRemoteWrite, Endpoint: srv.URL, PrometheusLabels: []string{"region"}}, FactoryConfig{})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if err := c.Send(context.Background(), testEvent()); err != nil {
		t.Fatal(err)
	}
	if ct != "application/x-protobuf" || enc != "snappy" || ver != "0.1.0" {
		t.Fatalf("headers")
	}
	if !strings.Contains(string(body), "request_duration") || !strings.Contains(string(body), "region") || strings.Contains(string(body), "environment") {
		t.Fatalf("payload %q", string(body))
	}
}
func TestPrometheusRemoteWriteRejectsNonMetricPermanently(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("should not send") }))
	defer srv.Close()
	c, err := Build(Spec{Kind: KindPrometheusRemoteWrite, Endpoint: srv.URL}, FactoryConfig{})
	if err != nil {
		t.Fatal(err)
	}
	e := testEvent()
	e.Value = nil
	err = c.Send(context.Background(), e)
	if err == nil || Retryable(err) {
		t.Fatalf("expected permanent")
	}
}
func TestVendorHealthProbesUseVendorCredentials(t *testing.T) {
	t.Setenv("TF_SPLUNK_HEALTH_TOKEN", "splunk-health")
	t.Setenv("TF_DD_HEALTH_KEY", "dd-health")
	var sa, dk string
	ss := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { sa = r.Header.Get("Authorization"); w.WriteHeader(200) }))
	defer ss.Close()
	ds := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { dk = r.Header.Get("DD-API-KEY"); w.WriteHeader(200) }))
	defer ds.Close()
	sp, err := Build(Spec{Kind: KindSplunkHEC, Endpoint: ss.URL, HealthEndpoint: ss.URL, APIKeyEnv: "TF_SPLUNK_HEALTH_TOKEN"}, FactoryConfig{})
	if err != nil {
		t.Fatal(err)
	}
	defer sp.Close()
	if err := sp.Ready(context.Background()); err != nil {
		t.Fatal(err)
	}
	dd, err := Build(Spec{Kind: KindDatadogLogs, Endpoint: ds.URL, HealthEndpoint: ds.URL, APIKeyEnv: "TF_DD_HEALTH_KEY"}, FactoryConfig{})
	if err != nil {
		t.Fatal(err)
	}
	defer dd.Close()
	if err := dd.Ready(context.Background()); err != nil {
		t.Fatal(err)
	}
	if sa != "Splunk splunk-health" || dk != "dd-health" {
		t.Fatalf("health auth %q %q", sa, dk)
	}
}
