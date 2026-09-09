# Tenant Isolation

Tenant identity is authorization data, not telemetry supplied by the client.

## Assignment

After authentication the gateway assigns:

```json
"tenant_id": "<principal tenant>"
```

to the canonical event.

A client request that supplies any `tenant_id` is rejected.

This removes ambiguity between "client metadata" and "authorization boundary."

## Kafka

Records are keyed by:

```text
tenant_id | source
```

This preserves source ordering inside a tenant while keeping tenants with the
same service name logically distinct.

Kafka headers also include:

```text
telemetryforge-tenant-id
```

## PostgreSQL

v1.0 tenant-scopes:

- event deduplication
- primary telemetry
- Flight Recorder
- incidents / incident events
- Cardinality Firewall findings
- shadow-policy differences
- quarantine evidence
- replay runs/results
- cost simulations
- Evidence Graph snapshots

Pre-v1 rows migrate to the explicit `default` tenant.

## Deduplication

Canonical event IDs are unique **within a tenant**:

```text
PRIMARY KEY (tenant_id, event_id)
```

Two tenants can therefore submit the same client-generated event ID without one
suppressing the other.

## In-memory state

Tenant isolation also applies before persistence.

The following keys include tenant identity:

- Cardinality Firewall estimator state
- cardinality finding suppression state
- shadow-difference suppression state
- automatic-incident error windows
- automatic-incident cooldowns

This prevents one tenant's traffic pattern from influencing another tenant's
policy state.

## CLI

Trusted operational commands accept:

```text
--tenant <tenant>
```

or:

```text
TELEMETRYFORGE_TENANT_ID
```

The tenant is placed into trusted internal context before any storage query.

## Current boundary

Tenant isolation is implemented inside one database/schema and Kafka cluster.

v1.0 does not claim cryptographic isolation between database tenants. Operators
still need database, broker, Kubernetes, and secret-manager access controls.
