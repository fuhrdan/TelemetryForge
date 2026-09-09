# Security Policy

## v0.8.0 security posture

TelemetryForge v0.8.0 adds Kubernetes/Terraform deployment material and the
Cardinality Firewall, but it remains a pre-1.0 engineering/portfolio release.

It should **not** yet be treated as an Internet-facing production multi-tenant
service.

Not yet implemented end to end:

- ingestion/dashboard authentication
- tenant isolation and authorization
- Kafka TLS/SASL configuration
- production secret-manager integration
- PII classification/redaction
- per-tenant policy/retention
- network policies/service mesh controls
- signed policy bundles
- production certificate/DNS automation

## Cardinality Firewall privacy

The firewall does not persist the raw high-cardinality value in its finding
table.

It stores a short SHA-256-derived fingerprint so operators can recognize
repeated observations without casually copying request/session/user identifiers
into a second operational dataset.

This fingerprinting is **not anonymization**. Low-entropy identifiers can still
be guessable, so access to finding tables should remain restricted.

Full values can still exist in:

- original telemetry
- the full-fidelity Flight Recorder
- explicit quarantine evidence
- frozen incidents

Those paths require the same or stronger access controls as the main telemetry
store.

## Quarantine

A `quarantine` policy preserves the original normalized event in a dedicated
TimescaleDB table, removes the dangerous dimension from the normal
representation, and marks the normal event as quarantined.

Quarantine evidence is idempotent by canonical event ID. Automated pruning is intentionally deferred until its replay/investigation horizon is defined.

## Policy files

Policies are executable operational configuration.

Treat changes to:

```text
policies/active.json
policies/shadow.json
```

as reviewed code changes. CI validates syntax/schema, but review is still needed
to understand whether a rule removes useful telemetry.

The shadow pipeline exists specifically to reduce the risk of promoting a
destructive candidate rule without evidence.

## Kubernetes secrets

`deployments/kubernetes/base/secret.example.yaml` contains only placeholders.

Real `secret.yaml` is Git-ignored. Production should normally use an approved
external secret manager / External Secrets / Sealed Secrets / cloud-native
secret workflow rather than committing Kubernetes Secret values.

## Terraform

The Terraform example creates billable AWS resources and does not configure a
complete security perimeter.

Before a real deployment, review:

- EKS endpoint exposure
- IAM access entries
- VPC endpoints
- network policies
- KMS/encryption requirements
- centralized Terraform state
- state encryption/access
- NAT/high-availability design
- audit logging

## Local credentials

Docker Compose values remain intentionally obvious development credentials.
Never reuse them outside local/demo environments.

## Reporting a vulnerability

Please avoid opening a public issue that contains exploit details, credentials,
or real telemetry. Use the repository's private security-reporting mechanism
when available.

Until then, contact the repository owner privately with enough information to
reproduce and assess the issue.
