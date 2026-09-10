package security

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAPIKeyScopeAndTenant(t *testing.T) {
	key := "tenant-alpha-reader"
	digest := sha256Hex(key)
	filename := filepath.Join(t.TempDir(), "keys.json")
	document := `{"keys":[{"name":"reader","sha256":"` + digest + `","tenant_id":"alpha","scopes":["read"]}]}`
	if err := os.WriteFile(filename, []byte(document), 0600); err != nil {
		t.Fatal(err)
	}

	auth, err := LoadAPIKeys(filename)
	if err != nil {
		t.Fatal(err)
	}

	var tenant string
	handler := auth.Middleware(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		tenant = TenantID(request.Context())
	}))

	request := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
	request.Header.Set("Authorization", "Bearer "+key)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if tenant != "alpha" {
		t.Fatalf("tenant=%q, want alpha", tenant)
	}
}

func TestReadOnlyKeyCannotIngest(t *testing.T) {
	key := "reader"
	filename := filepath.Join(t.TempDir(), "keys.json")
	document := `{"keys":[{"name":"reader","sha256":"` + sha256Hex(key) + `","tenant_id":"alpha","scopes":["read"]}]}`
	if err := os.WriteFile(filename, []byte(document), 0600); err != nil {
		t.Fatal(err)
	}
	auth, err := LoadAPIKeys(filename)
	if err != nil {
		t.Fatal(err)
	}

	handler := auth.Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	request := httptest.NewRequest(http.MethodPost, "/api/v1/events", strings.NewReader("{}"))
	request.Header.Set("X-TelemetryForge-Key", key)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want %d", response.Code, http.StatusForbidden)
	}
}

func TestHealthRemainsPublic(t *testing.T) {
	auth := &Authenticator{enabled: true}
	handler := auth.Middleware(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health", nil))
	if response.Code != http.StatusNoContent {
		t.Fatalf("status=%d", response.Code)
	}
}

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func TestAPIKeyFileRejectsTrailingJSON(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "keys.json")
	document := `{"keys":[{"name":"reader","sha256":"` + sha256Hex("reader") + `","tenant_id":"alpha","scopes":["read"]}]} {}`
	if err := os.WriteFile(filename, []byte(document), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadAPIKeys(filename); err == nil {
		t.Fatal("expected trailing JSON value to be rejected")
	}
}

func TestAdminDoesNotImplyControl(t *testing.T) {
	p := Principal{Scopes: map[string]struct{}{ScopeAdmin: {}}}
	if p.Has(ScopeControl) {
		t.Fatal("admin must not imply global control scope")
	}
	p.Scopes[ScopeControl] = struct{}{}
	if !p.Has(ScopeControl) {
		t.Fatal("explicit control scope should be honored")
	}
}
