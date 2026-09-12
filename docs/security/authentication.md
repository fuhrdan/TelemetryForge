# Authentication and Authorization

TelemetryForge v1.0.0 adds an explicit gateway authentication boundary.

## Modes

### `disabled`

Local-development default.

The gateway injects one trusted principal for:

```text
tenant_id = default
scope = admin
```

Do not expose this mode to an untrusted network.

### `api_key`

Production-oriented API-key authentication.

Enable:

```text
TELEMETRYFORGE_AUTH_MODE=api_key
TELEMETRYFORGE_API_KEYS_FILE=/var/run/secrets/telemetryforge/api-keys.json
```

## API-key file

TelemetryForge stores **SHA-256 digests**, not raw API keys:

```json
{
  "keys": [
    {
      "name": "dashboard-read",
      "sha256": "<64 hex characters>",
      "tenant_id": "production",
      "scopes": ["read"]
    }
  ]
}
```

The raw key belongs in the client/secret manager only.

Generate a key and digest with your approved secret tooling. One simple local
example is:

```bash
KEY="$(openssl rand -hex 32)"
printf '%s' "$KEY" | sha256sum
```

Do not commit `KEY`.

## Scopes

### `ingest`

Allows:

```text
POST /api/v1/events
POST /api/v1/metrics
```

### `read`

Allows tenant-scoped `/api/*` reads, including:

- telemetry queries
- dashboard summaries
- incidents
- Cardinality Firewall findings
- replay/cost history
- Evidence Graph

### `admin`

Implies all scopes and permits administrative surfaces such as:

```text
GET /metrics
```

## Headers

Preferred:

```text
Authorization: Bearer <key>
```

Also supported for simple ingestion clients:

```text
X-TelemetryForge-Key: <key>
```

## Probe endpoints

These remain unauthenticated:

```text
GET /health
GET /ready
```

Kubernetes can therefore probe the process before application credentials are
available to a probe agent.

## Dashboard

The browser never receives the gateway read key.

The Next.js server-side `/telemetry-api/*` proxy reads:

```text
TELEMETRYFORGE_DASHBOARD_API_KEY
```

and injects it into the server-to-gateway request.

See `docs/security/dashboard-proxy.md`.


## Internal edge mesh authentication

The v2.3 edge mesh uses a separate bearer token, `TELEMETRYFORGE_MESH_TOKEN`, for `/internal/v1/mesh/*`. These endpoints bypass tenant API-key authentication because they are node-to-node transport endpoints. Keep them on private networks and store the token in a secret manager. The replication token and mesh token may be rotated independently.
