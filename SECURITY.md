# Security Policy

## v1.5.0 security posture

TelemetryForge retains the v1 application-level security boundaries and v1.5 adds portable incident evidence controls:

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


## Schema Intelligence

The v1.1 schema registry stores:

- tenant/source/type/version identity;
- field paths and JSON types;
- counts/timestamps;
- semantic-convention metadata; and
- schema fingerprints derived from field paths/types.

It does **not** put raw telemetry values into schema fingerprints or registry
field definitions.

Field names can still reveal business concepts, so schema APIs require the same
tenant-scoped `read` authorization as other operational evidence.

`schema_url` is treated as metadata only. TelemetryForge v1.1 does not fetch or
dereference client-supplied schema URLs, avoiding an SSRF-style network fetch
boundary.


## Schema-registry resource bounds

Schema Intelligence never stores telemetry values in schema fingerprints, but
producer-controlled field names are still untrusted metadata.

v1.1.0 therefore caps each accumulated schema version at 2,048 unique field
paths and caps per-event discovery at 512 paths / five payload levels. This
prevents dynamic key generation from creating unbounded registry state.


## Distributed cardinality privacy

v1.2 shared cardinality state does not persist raw tag values.

It stores:

- fixed HLL register bytes;
- up to 16 signed 64-bit representations of SHA-256-derived hashes; and
- aggregate timing/count metadata.

The short finding fingerprint and stored hashes are **not anonymization**. As
with earlier finding fingerprints, low-entropy values can be dictionary-tested.
Database access should remain restricted.

Series-budget identities are assembled in process and hashed before shared
state persistence; the full series string is not stored in the cardinality
state table.


## Telemetry Router destinations

Routing policy is operational configuration and should be reviewed like policy.

HTTP destination secrets must not be stored directly in routing JSON. Static
`Authorization` and `Cookie` headers are rejected. Use `bearer_token_env` and a
secret manager/environment injection for bearer credentials.

The router sends the canonical post-policy event, which may still contain
sensitive telemetry. Destination access, TLS, retention, and downstream tenant
isolation remain deployment responsibilities.

Per-destination dead letters persist the event envelope for recovery and are
therefore sensitive operational evidence.


## Telemetry Router destination credentials

HTTP destination credentials must not be embedded directly in routing JSON.
Credential-shaped static headers are rejected. Use `header_env` or
`bearer_token_env` and supply their values through the deployment secret
mechanism. Routing dead letters retain full event envelopes and therefore require
the same access controls as frozen incident evidence.


## Portable incident archives

`.tfincident` files can contain full-fidelity frozen telemetry, schema history,
Evidence Graph relationships, and configuration snapshots.

Treat an unencrypted archive as sensitive production evidence.

Optional encrypted archives use AES-256-GCM over the complete ZIP payload. The
key must be a random 32-byte value represented as 64 hexadecimal characters.

TelemetryForge does not accept human passwords for archive encryption in v1.5;
it therefore avoids silently introducing a weak or undocumented password KDF.

Archive verification rejects:

- changed members;
- extra/unlisted members;
- missing members;
- duplicate ZIP paths;
- path traversal;
- incompatible format versions;
- count mismatches;
- oversized archive/member/event content.

Import never activates archived configuration or sends imported evidence through
the production pipeline.

Cross-tenant import requires an explicit operator acknowledgement.

The following local artifacts are Git-ignored by default:

```text
*.tfincident
*.tfincident.enc
*.archive.key
incident-exports/
```

Standalone HTML reports are also sensitive if they summarize real production
evidence, even though they contain no external scripts/assets.

## Connector credentials

Connector configuration stores secret references (`header_env`, `bearer_token_env`, `api_key_env`) rather than raw secrets. Credential-bearing static headers and URL credentials are rejected. Prometheus labels require explicit allowlisting. OTLP protobuf ingestion is explicitly unsupported in v1.8 rather than partially decoded.
