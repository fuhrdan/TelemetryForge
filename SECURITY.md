# Security Policy

## v0.9.0 security posture

TelemetryForge v0.9.0 has strong durability/policy/replay boundaries, but it is
still a pre-1.0 engineering/portfolio release rather than a fully hardened
Internet-facing multi-tenant service.

Not yet implemented end to end:

- ingestion/dashboard authentication
- tenant isolation and authorization
- production Kafka TLS/SASL profiles
- production secret-manager integration
- full PII classification/redaction
- per-tenant policy/retention
- hardened observability authentication
- Kubernetes NetworkPolicy/service-mesh controls
- signed policy/pricing bundles

## Incident Replay safety

Replay is analysis-only by default.

It does not write to:

- primary telemetry storage
- Flight Recorder
- automatic incident capture
- production Kafka ingest topics

Optional Kafka output is restricted by both the replay library and
`telemetryctl` to:

```text
telemetry.replay
telemetry.replay.<suffix>
```

No normal checked-in worker consumes that namespace.

A future connector must preserve this isolation rather than treating replay
records as ordinary production telemetry.

## Cost pricing input

Pricing JSON is operational/configuration input, not trusted telemetry.

Dollar projections occur only with explicit non-zero pricing fields and a
currency.

Pricing models should be reviewed/versioned because they can influence business
decisions even though they do not affect telemetry execution.

## Cardinality Firewall privacy

The Cardinality Firewall does not persist the raw high-cardinality tag value in
its finding table.

It stores a short SHA-256-derived fingerprint. That reduces unnecessary copying
but is **not anonymization**; low-entropy values can still be guessable.

Full identifiers can exist in:

- original telemetry
- Flight Recorder
- frozen incidents
- quarantine evidence

Those data paths require strict access controls.

## Metrics endpoints

The gateway `/metrics` and worker admin `/metrics` endpoints expose operational
state.

The checked-in Prometheus configuration is a local development example and does
not authenticate these endpoints.

Do not expose them directly to untrusted networks.

Prometheus labels intentionally avoid event IDs, incident IDs, correlation IDs,
raw URL paths, and arbitrary telemetry tags.

## OpenTelemetry / Tempo

The local Collector and Tempo endpoints are unauthenticated development
services.

Docker Compose publishes OTLP ports to localhost for convenient testing. Tempo
uses local filesystem storage.

Production should use authenticated/encrypted telemetry transport, restricted
network paths, and an appropriate production trace backend/object store.

Trace attributes can still contain telemetry source/event type. Traces remain
operationally sensitive even though those attributes are not Prometheus labels.

## Grafana

Local credentials:

```text
admin / telemetryforge
```

They are intentionally obvious development credentials and must never be reused
outside local/demo environments.

## Kubernetes secrets

`deployments/kubernetes/base/secret.example.yaml` contains placeholders only.

Real `secret.yaml` is Git-ignored. Production should use the organization's
approved secret-management mechanism.

## Terraform

Terraform creates billable resources and state can contain sensitive
infrastructure metadata.

Production review should include:

- remote encrypted state
- access control
- EKS endpoint exposure
- IAM access entries
- KMS/encryption requirements
- VPC endpoints/network policy
- audit logging
- availability design

## Reporting a vulnerability

Do not open a public issue containing exploit details, credentials, or real
telemetry.

Use GitHub private security reporting when enabled, or contact the repository
owner privately with reproduction information.
