# TelemetryForge v1.0.0 Release Notes

## Evidence-backed incident control plane

v1.0.0 completes the original TelemetryForge portfolio arc: ingest telemetry
durably, preserve incident evidence, protect downstream observability systems,
evaluate policy safely, replay incidents, estimate telemetry impact, explain
captured relationships, and expose explicit production security boundaries.

## Evidence Graph

Added an explainable graph generated from frozen incidents.

Relationships are classified as:

```text
supporting
contradicting
related
```

The graph can connect evidence through:

- correlation ID
- trace ID
- source/time sequence
- latency-before-error
- deployment/change-before-error
- recovery signal
- replay runs
- cost simulations

A change/error temporal relationship explicitly says that it **does not prove
causality**.

The dashboard shows graph summary, hypotheses, supporting evidence, and
contradictory evidence.

## Authentication and authorization

Added API-key authentication with hash-only server configuration.

Scopes:

```text
ingest
read
admin
```

`admin` implies all scopes.

Health/readiness remain unauthenticated for orchestrator probes.

## Tenant isolation

Tenant identity is assigned from authentication.

Client-supplied `tenant_id` is rejected.

Tenant boundaries now cover:

- Kafka partition key/header
- event deduplication
- telemetry queries
- Flight Recorder/incidents
- Cardinality Firewall state/findings
- automatic incident state
- quarantine
- replay/cost history
- Evidence Graph

Existing pre-v1 data migrates to the `default` tenant.

## Dashboard credential boundary

The browser no longer depends on a direct gateway rewrite.

A server-side Next.js `/telemetry-api/*` proxy injects a tenant-scoped read-only
credential, keeping the raw key out of browser JavaScript and supporting
authenticated SSE.

## Redaction

Added configurable presentation-time tag redaction and optional payload
suppression.

Stored Flight Recorder/frozen evidence remains full fidelity.

## Kafka security

Added:

- TLS 1.2+
- custom CA
- optional mTLS client certificate
- SASL PLAIN
- SCRAM-SHA-256
- SCRAM-SHA-512

Gateway, worker, and telemetryctl use the same environment contract.

## Kubernetes production profile

Added a production Kustomize overlay with:

- `api_key` auth
- secret-mounted hash document
- dashboard read credential
- payload redaction
- Kafka TLS/SCRAM
- non-root/seccomp
- disabled service-account token automount
- authenticated metrics guidance

## Operations

Added:

- v1 upgrade runbook
- v1 rollback runbook
- portfolio demo guide
- production profile guide

## Compatibility

Pre-v1 data receives:

```text
tenant_id = default
```

The local disabled-auth Compose profile also uses `default`, preserving existing
local data visibility after migration.

## Validation philosophy

v1.0.0 does not claim that configuration files alone create a universally
secure production system.

The repository documents which controls are implemented and which remain
deployment/operator responsibilities.
