# Security Policy

## v1.0.0 security posture

v1.0.0 implements the major application-level portfolio security boundaries:

- scoped API-key authentication
- authentication-derived tenant identity
- tenant-scoped storage and in-memory policy state
- server-side dashboard credentials
- read/export redaction
- Kafka TLS/mTLS/SASL
- protected metrics in authenticated mode
- hardened Kubernetes pod defaults
- production deployment overlay

Security still depends on the deployment around the application.

## Authentication

Production should use:

```text
TELEMETRYFORGE_AUTH_MODE=api_key
```

The API-key document stores SHA-256 digests only.

Scopes:

```text
ingest
read
admin
```

`admin` implies all scopes.

The raw dashboard read key belongs only in the dashboard server secret.

## Tenant isolation

Clients cannot choose their tenant.

Any input event containing `tenant_id` is rejected; the gateway assigns the
tenant from authentication.

The tenant then follows the event through Kafka and tenant-scoped persistence.

Tests cover:

- client tenant spoof rejection;
- same event ID used independently by two tenants;
- separate in-memory Cardinality Firewall state.

## Full-fidelity evidence

Presentation-time redaction does **not** erase stored Flight Recorder/incident
evidence.

Protect:

- PostgreSQL/TimescaleDB
- backups
- direct SQL access
- quarantine evidence
- frozen incidents

Organizations prohibited from storing specific PII need an ingestion-time
redaction policy before those fields reach TelemetryForge.

## Dashboard

The browser does not receive the gateway API key.

Next.js proxies `/telemetry-api/*` server-side and injects a read-only key.

Do not use an admin key for the dashboard.

## Kafka

Supported security configuration:

- TLS 1.2+
- custom CA
- optional mTLS
- SASL PLAIN
- SCRAM-SHA-256
- SCRAM-SHA-512

Production should prefer encrypted transport and the broker's approved
authentication mechanism.

## Metrics and tracing

When gateway auth is enabled:

```text
GET /metrics
```

requires `admin`.

The production overlay disables anonymous scrape annotations. Configure
Prometheus with a bearer credential or equivalent cluster-specific mechanism.

OTLP/Tempo/Grafana examples in local Compose are development surfaces, not
production authentication examples.

## Secrets

Do not commit:

- raw API keys
- database URLs containing credentials
- Kafka SASL passwords
- private keys/client certificates
- Terraform state

Use the organization's approved secret manager.

## Reporting

Do not open a public issue containing credentials, exploit details, or real
telemetry.

Use GitHub private security reporting when enabled or contact the repository
owner privately.
