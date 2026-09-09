# TelemetryForge Architecture

TelemetryForge v1.2.0 is an OpenTelemetry-native telemetry control plane built
around durability, bounded concurrency, evidence preservation, reversible
policy, replayable investigations, and explicit tenant/security boundaries.

## Runtime path

```text
Authenticated client
       |
       v
Go Gateway
  assign tenant_id
       |
       v
Kafka (tenant|source key)
       |
       v
Bounded Worker Group
       |
       v
Flight Recorder (full fidelity)
       |
       v
Normalizer
       |
       v
Schema Intelligence
       |
       v
Cardinality Firewall
  active + shadow
       |
       +--> shared hourly HLL / series budgets
       |
       v
TimescaleDB
       |
       +------> Frozen Incident
       |           |
       |           +--> Incident Replay
       |           +--> Cost Simulator
       |           `--> Evidence Graph
       |
       `------> Tenant-scoped Query/SSE API
                       |
                       v
               Next.js server proxy
                       |
                       v
                    Browser
```

Gateway and worker also expose bounded Prometheus metrics and sampled
OpenTelemetry traces.

## Core guarantees

1. **Durable acceptance.** HTTP `202` follows Kafka acknowledgement.
2. **Bounded memory.** Worker queues, incident state, and cardinality state have
   configured bounds.
3. **At-least-once processing.** Offsets advance after successful or durable DLQ
   handling.
4. **Contiguous partition commits.** Later worker completion cannot commit past
   earlier unfinished records in one partition.
5. **Rebalance-safe poll batches.** Partition ownership remains stable while a
   bounded batch drains.
6. **Tenant-local idempotency.** `(tenant_id,event_id)` prevents duplicate
   processing without cross-tenant suppression.
7. **Evidence before mutation.** Flight Recorder capture precedes policy changes.
8. **Shadow before promotion.** Candidate policy does not change active behavior.
9. **Replay isolation.** Incident Replay is analysis-only unless explicitly
   exported to `telemetry.replay*`.
10. **No implicit pricing.** Cost dollars require explicit pricing input.
11. **Evidence association is not causality.** Graph edges carry basis,
    assessment, and contradictory evidence.
12. **Tenant identity comes from authentication.** Client-supplied tenant IDs
    are rejected.

## Authentication / tenant propagation

In `api_key` mode the gateway authenticates a hash-backed API key and resolves:

```text
principal name
tenant_id
scopes
```

The canonical event receives `tenant_id` only after authentication.

Kafka key:

```text
tenant_id | source
```

Kafka header:

```text
telemetryforge-tenant-id
```

Worker-side policy/incident state uses the event tenant rather than HTTP
context, so tenant isolation survives the asynchronous broker boundary.

## Storage boundary

Tenant-aware data includes:

- event deduplication
- telemetry events
- Flight Recorder
- incidents
- policy findings/diffs
- quarantine evidence
- replay history/results
- cost simulations
- Evidence Graph snapshots

Pre-v1 rows migrate to tenant `default`.

## Schema Intelligence

v1.1 observes normalized telemetry before policy mutation and records a
versioned, tenant-scoped source/type schema.

The registry tracks field paths/types, observation counts, inferred-required
fields, semantic-convention findings, application `schema_version`, and optional
OpenTelemetry `schema_url`.

The observation is idempotent by `(tenant_id,event_id)` and fail-open by
default, so worker retries cannot inflate counts and a registry outage does not
become a telemetry outage.

Schema drift does not mutate or reject the event.


## Distributed Cardinality Intelligence

Production workers use a shared TimescaleDB-backed tracker keyed by tenant,
active/shadow mode, source, event type, dimension, and one-hour UTC window.

Each update raises one of 64 HLL registers with an atomic component-wise
maximum. The first 16 SHA-256-derived unique hashes are retained exactly for
small-threshold accuracy. Raw tag values are never stored in shared cardinality
state.

Versioned policy can also define tenant/source series budgets. A budget hashes
the full series identity (source + event type + sorted non-control tags), so its
consumption is not the invalid sum of individual label cardinalities.

Budgets are advisory in v1.2.0. Only explicit dimension rules can mutate an
event.

Replay and cost analysis use isolated local trackers with the same hourly
windows and never write the production cardinality state.

See:

- `docs/cardinality/distributed-cardinality.md`
- `docs/cardinality/budgets.md`

## Evidence Graph

Graph construction uses captured fields only.

Supporting examples:

- same correlation ID
- same trace ID
- latency before error
- captured change before error

Contradicting example:

- recovery signal after an error contradicting uninterrupted degradation

Context-only example:

- same source within a short time window

The graph does not label any node as an automated root cause.

See `docs/evidence/evidence-graph.md`.

## Dashboard security boundary

The browser talks to:

```text
/telemetry-api/*
```

on the Next.js application.

The Next.js server injects a tenant-scoped read-only gateway credential from its
server environment.

This keeps the reusable key out of browser JavaScript and supports authenticated
SSE.

## Read-time redaction

The gateway can redact configured tags and/or payloads on:

- query API
- frozen incident API
- live SSE

Stored evidence remains unchanged.

This preserves forensic evidence but means direct database/backups remain
sensitive.

## Kafka security

The same security profile is used by gateway, worker, and telemetryctl.

Supported:

```text
TLS 1.2+
custom CA
optional mTLS client certificate
SASL PLAIN
SCRAM-SHA-256
SCRAM-SHA-512
```

Local Compose remains plaintext for development.

## Self-observability

Prometheus labels stay bounded.

Trace Context/Baggage propagates through Kafka, linking:

```text
HTTP -> Kafka produce -> Kafka consume -> worker.process -> worker.stage
```

Kafka lag comes from committed group offsets versus broker end offsets.

## Deployment profiles

### Local

`docker compose up --build`

- auth disabled
- tenant `default`
- development database/broker credentials
- local Grafana/Prometheus/Tempo

### Kubernetes base

Readable application deployment with probes, resources, HPA, PDB, and policy
ConfigMap.

### Kubernetes production overlay

Adds:

- API-key auth
- secret-mounted key hashes
- server-side dashboard read key
- payload redaction
- Kafka TLS/SCRAM
- non-root/seccomp
- disabled service-account token automount
- authenticated metrics guidance

See `docs/deployment/production-profile.md`.

## Evidence-first future extensions

Any future AI explanation layer should consume Evidence Graph nodes/edges,
reference its supporting evidence, and preserve contradictory evidence rather
than replacing the graph with an opaque root-cause score.
