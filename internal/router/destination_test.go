package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
)

func TestHTTPSenderPostsCanonicalEnvelopeAndIdempotencyKey(t *testing.T) {
	var gotID string
	var gotTenant string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		gotID = request.Header.Get("Idempotency-Key")
		gotTenant = request.Header.Get("X-TelemetryForge-Tenant-ID")
		writer.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	sender, err := newHTTPSender(withDestinationDefaults(Destination{Name: "webhook", Type: DestinationHTTP, Enabled: true, URL: server.URL}))
	if err != nil {
		t.Fatal(err)
	}
	event := domain.Event{ID: "evt-1", TenantID: "acme", Source: "checkout", Type: "request"}
	if err := sender.Send(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if gotID != "evt-1" || gotTenant != "acme" {
		t.Fatalf("headers id=%q tenant=%q", gotID, gotTenant)
	}
}

func TestHTTPSenderTreatsNon2xxAsFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, "no", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	sender, err := newHTTPSender(withDestinationDefaults(Destination{Name: "webhook", Type: DestinationHTTP, Enabled: true, URL: server.URL}))
	if err != nil {
		t.Fatal(err)
	}
	if err := sender.Send(context.Background(), domain.Event{ID: "evt"}); err == nil {
		t.Fatal("expected HTTP failure")
	}
}

func TestRetryDelayIsBounded(t *testing.T) {
	if got := retryDelay(10, 500, 30000); got != 30000000000 {
		t.Fatalf("delay=%v", got)
	}
}

func TestHTTPSenderLoadsGenericSecretHeaderFromEnvironment(t *testing.T) {
	t.Setenv("TF_ROUTER_TEST_API_KEY", "secret-from-environment")
	var got string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		got = request.Header.Get("X-API-Key")
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	sender, err := newHTTPSender(withDestinationDefaults(Destination{
		Name: "webhook", Type: DestinationHTTP, Enabled: true, URL: server.URL,
		HeaderEnv: map[string]string{"X-API-Key": "TF_ROUTER_TEST_API_KEY"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if err := sender.Send(context.Background(), domain.Event{ID: "evt"}); err != nil {
		t.Fatal(err)
	}
	if got != "secret-from-environment" {
		t.Fatalf("X-API-Key=%q", got)
	}
}

func TestHTTPSenderRejectsMissingSecretEnvironmentAtStartup(t *testing.T) {
	_, err := newHTTPSender(withDestinationDefaults(Destination{
		Name: "webhook", Type: DestinationHTTP, Enabled: true, URL: "https://example.invalid",
		HeaderEnv: map[string]string{"X-API-Key": "TF_ROUTER_MISSING_SECRET"},
	}))
	if err == nil {
		t.Fatal("expected missing header environment to fail sender initialization")
	}
}
