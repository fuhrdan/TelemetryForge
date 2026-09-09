// Package security implements TelemetryForge authentication, authorization,
// tenant context, and security-oriented HTTP middleware.
package security

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

type contextKey string

const principalKey contextKey = "telemetryforge-principal"

const (
	ScopeIngest = "ingest"
	ScopeRead   = "read"
	ScopeAdmin  = "admin"
)

// Principal is the authenticated identity attached to a request.
//
// TenantID is authorization data. Clients cannot choose another tenant by
// placing a tenant ID in telemetry or a query string.
type Principal struct {
	Name     string
	TenantID string
	Scopes   map[string]struct{}
}

// Has reports whether the principal owns a scope. Admin implies all scopes.
func (principal Principal) Has(scope string) bool {
	if _, ok := principal.Scopes[ScopeAdmin]; ok {
		return true
	}
	_, ok := principal.Scopes[scope]
	return ok
}

// WithPrincipal returns a context carrying one authenticated principal.
func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalKey, principal)
}

// PrincipalFrom returns the authenticated principal, if any.
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalKey).(Principal)
	return principal, ok
}

// TenantID returns the current tenant and deliberately falls back to "default"
// for non-HTTP internal/test contexts.
//
// Production HTTP requests are authenticated before reaching storage. Worker
// writes carry tenant_id in the canonical event itself.
func TenantID(ctx context.Context) string {
	if principal, ok := PrincipalFrom(ctx); ok {
		if tenant := strings.TrimSpace(principal.TenantID); tenant != "" {
			return tenant
		}
	}
	return "default"
}

// WithTenant is used by trusted internal/CLI workflows that already resolved a
// tenant and need storage queries to enforce the same boundary.
func WithTenant(ctx context.Context, tenantID string) context.Context {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		tenantID = "default"
	}
	return WithPrincipal(ctx, Principal{
		Name:     "internal",
		TenantID: tenantID,
		Scopes: map[string]struct{}{
			ScopeAdmin: {},
		},
	})
}

type keyDocument struct {
	Keys []keyEntry `json:"keys"`
}

type keyEntry struct {
	Name     string   `json:"name"`
	SHA256   string   `json:"sha256"`
	TenantID string   `json:"tenant_id"`
	Scopes   []string `json:"scopes"`
}

type credential struct {
	hash      [32]byte
	principal Principal
}

// Authenticator validates API keys without storing their raw value.
type Authenticator struct {
	enabled       bool
	defaultTenant string
	keys          []credential
}

// Disabled returns a local-development authenticator that injects one trusted
// principal. This mode must not be used for an exposed production gateway.
func Disabled(defaultTenant string) *Authenticator {
	defaultTenant = strings.TrimSpace(defaultTenant)
	if defaultTenant == "" {
		defaultTenant = "default"
	}
	return &Authenticator{
		enabled:       false,
		defaultTenant: defaultTenant,
	}
}

// LoadAPIKeys loads a strict JSON document containing SHA-256 API-key hashes.
func LoadAPIKeys(filename string) (*Authenticator, error) {
	payload, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("read API key file: %w", err)
	}

	var document keyDocument
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("decode API key file: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, errors.New("API key file contains a trailing JSON value")
		}
		return nil, fmt.Errorf("decode API key file trailing data: %w", err)
	}
	if len(document.Keys) == 0 {
		return nil, errors.New("API key file contains no keys")
	}

	auth := &Authenticator{enabled: true}
	seen := make(map[[32]byte]struct{}, len(document.Keys))
	for index, entry := range document.Keys {
		name := strings.TrimSpace(entry.Name)
		tenantID := strings.TrimSpace(entry.TenantID)
		if name == "" || tenantID == "" {
			return nil, fmt.Errorf("API key entry %d requires name and tenant_id", index)
		}
		decoded, err := hex.DecodeString(strings.TrimSpace(entry.SHA256))
		if err != nil || len(decoded) != sha256.Size {
			return nil, fmt.Errorf("API key entry %d has invalid SHA-256 digest", index)
		}

		var digest [32]byte
		copy(digest[:], decoded)
		if _, exists := seen[digest]; exists {
			return nil, fmt.Errorf("duplicate API key digest at entry %d", index)
		}
		seen[digest] = struct{}{}

		scopes := make(map[string]struct{}, len(entry.Scopes))
		for _, scope := range entry.Scopes {
			scope = strings.TrimSpace(scope)
			switch scope {
			case ScopeIngest, ScopeRead, ScopeAdmin:
				scopes[scope] = struct{}{}
			default:
				return nil, fmt.Errorf("API key entry %d has unsupported scope %q", index, scope)
			}
		}
		if len(scopes) == 0 {
			return nil, fmt.Errorf("API key entry %d requires at least one scope", index)
		}

		auth.keys = append(auth.keys, credential{
			hash: digest,
			principal: Principal{
				Name:     name,
				TenantID: tenantID,
				Scopes:   scopes,
			},
		})
	}
	return auth, nil
}

// Middleware authenticates protected API/admin requests and injects tenant
// authorization context. /health and /ready remain unauthenticated so
// orchestrator probes do not require application credentials.
func (auth *Authenticator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		setSecurityHeaders(writer)

		if request.URL.Path == "/health" || request.URL.Path == "/ready" {
			next.ServeHTTP(writer, request)
			return
		}

		required := requiredScope(request)
		principal, ok := auth.authenticate(request)
		if !ok {
			writeAuthError(writer, http.StatusUnauthorized, "authentication required")
			return
		}
		if !principal.Has(required) {
			writeAuthError(writer, http.StatusForbidden, "insufficient scope")
			return
		}

		next.ServeHTTP(writer, request.WithContext(WithPrincipal(request.Context(), principal)))
	})
}

func (auth *Authenticator) authenticate(request *http.Request) (Principal, bool) {
	if auth == nil || !auth.enabled {
		tenant := "default"
		if auth != nil && auth.defaultTenant != "" {
			tenant = auth.defaultTenant
		}
		return Principal{
			Name:     "local-development",
			TenantID: tenant,
			Scopes: map[string]struct{}{
				ScopeAdmin: {},
			},
		}, true
	}

	raw := bearerToken(request.Header.Get("Authorization"))
	if raw == "" {
		raw = strings.TrimSpace(request.Header.Get("X-TelemetryForge-Key"))
	}
	if raw == "" {
		return Principal{}, false
	}

	digest := sha256.Sum256([]byte(raw))
	for _, candidate := range auth.keys {
		if subtle.ConstantTimeCompare(digest[:], candidate.hash[:]) == 1 {
			return candidate.principal, true
		}
	}
	return Principal{}, false
}

func requiredScope(request *http.Request) string {
	if request.URL.Path == "/metrics" {
		return ScopeAdmin
	}
	if request.Method == http.MethodPost &&
		(request.URL.Path == "/api/v1/events" || request.URL.Path == "/api/v1/metrics") {
		return ScopeIngest
	}
	if strings.HasPrefix(request.URL.Path, "/api/") {
		return ScopeRead
	}
	return ScopeAdmin
}

func bearerToken(header string) string {
	const prefix = "Bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(header[len(prefix):])
}

func setSecurityHeaders(writer http.ResponseWriter) {
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.Header().Set("X-Frame-Options", "DENY")
	writer.Header().Set("Referrer-Policy", "no-referrer")
	writer.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
}

func writeAuthError(writer http.ResponseWriter, status int, message string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(map[string]string{"error": message})
}
