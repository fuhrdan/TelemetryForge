# TelemetryForge Architecture

TelemetryForge v2.2.0 is an OpenTelemetry-native telemetry control plane built
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
Change Intelligence
       |
       v
Adaptive Shaping
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

1. **Explicit durable acceptance boundary.** The gateway returns success after Kafka acknowledgement. The edge fsyncs locally first; configured non-local durability modes also require a quorum satisfying the requested zone/region/cloud failure-domain rule before success.
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
13. **Change association is not causality.** Deployment-before-failure and rollback-before-recovery relationships remain observational evidence.
14. **Edge backpressure before overwrite.** The edge rejects new acceptance when WAL capacity cannot contain the next record.
15. **Corruption fails closed.** Only an incomplete final WAL frame is auto-truncated; completed-frame integrity failure blocks recovery.
16. **Durability class cannot silently degrade.** If live peers cannot satisfy the configured quorum/failure-domain rule, edge readiness and successful acceptance fail instead of falling back to a weaker class.
17. **Replica acknowledgement follows fsync.** Receiving peers acknowledge only after the origin record is durably appended to the replica log.

## Durable Edge boundary

v2.1 adds the local WAL foundation in front of Kafka. v2.2 can extend that boundary across peers: the origin fsyncs first, peers persist the original WAL record into fsynced replica logs, and the requested durability mode evaluates both quorum size and failure-domain diversity before successful acceptance. Replay re-checks replication before Kafka delivery.

See `docs/edge/durable-edge.md` and `docs/edge/replicated-durability.md`.

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
- change markers / analysis snapshots

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

## Change Intelligence

v1.7 records normalized deployment, rollback, release, feature-flag, configuration, and infrastructure markers before lossy shaping. The change marker carries source, environment, version lineage, Git SHA, build ID, actor, and explicit rollback relationships.

Before/after analysis reads bounded tenant telemetry windows and compares event count, error rate, and p95 latency per observed source. Sources with sparse evidence remain `insufficient`. The reported blast radius is the share of assessable observed sources that materially regressed; it is not a topology-derived causal radius.

Structured changes also enrich the Evidence Graph and portable incident archive. Rollback/recovery relationships preserve explicit non-causality language.

See `docs/change-intelligence/change-intelligence.md`.

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

## Portable Incident Archive

A `.tfincident` is a portable copy of **frozen evidence**, not another live
pipeline.

```text
Frozen Incident
    |
    +--> event/capture timeline
    +--> Evidence Graph
    +--> replay/cost history
    +--> relevant schema history/drift
    +--> policy/shaping/routing snapshots
    |
    v
manifest + SHA-256 member checksums
    |
    +--> ordinary .tfincident ZIP
    `--> optional AES-256-GCM outer envelope
```

Archive import writes only a frozen incident plus provenance. It deliberately
does not call ingestion, primary persistence, adaptive sampling, Cardinality
Firewall processing, or routing.

Configuration snapshots remain historical evidence and are never activated by
import.

The parser bounds member count, individual member size, total uncompressed
bytes, event count, JSONL line size, and ZIP member paths.

## Operational Proof plane

v1.9 adds a separate evidence loop around the running control plane:

```text
executed scenario
   |
   +--> raw command / k6 / DB / Kubernetes evidence
   |
   v
.tfproof.json
   |  Git commit
   |  environment
   |  configuration SHA-256
   |  measurements
   |  assertions
   |  raw evidence SHA-256
   v
telemetryctl verify / record
   |
   v
operational_proof_runs --> API --> dashboard
```

Proof tooling does not sit on the production event path. It observes or
intentionally perturbs approved test infrastructure and records what happened.

Kubernetes availability hardening complements the proof loop with explicit
RollingUpdate, PDB, HPA, topology-spread, and readiness semantics.

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


## Adaptive Sampling & Telemetry Shaping

The production order preserves evidence before any lossy shaping:

```text
Flight Recorder
  -> Normalizer
  -> Schema Intelligence
  -> Adaptive Sampling / Shaping
  -> Distributed Cardinality Firewall
  -> Primary persistence
  -> Routing outbox
```

Sampled-out events are successful processing outcomes. They are acknowledged
after shaping evidence is durably recorded and do not enter the processing DLQ.

Errors, severe events, high-latency signals, audit/deployment events, and
incident-tagged telemetry are protected by default.

Candidate shaping runs in shadow mode and never changes active keep/drop
behavior.

## Telemetry Router

The primary worker records post-policy delivery intent in `routing_deliveries`
after primary persistence. External backend I/O is owned by the separate
`telemetryforge-router` process.

```text
Kafka processing worker
  -> primary persistence
  -> routing outbox
  -> source ack

routing outbox
  -> destination-specific leased workers
       -> delivered
       -> retry
       -> per-destination DLQ
       -> optional failure fallback
```

Each destination advances independently. Router replicas use
`FOR UPDATE SKIP LOCKED` plus an expiring lease so one claimed delivery can be
recovered after process failure.

Shadow routing evaluates candidate destination sets but never writes candidate
outbox rows.

Delivery to external backends is at-least-once. Kafka destination keys retain
`tenant_id|source`; HTTP destinations receive the event ID as an idempotency
key.

## Connector Platform

The routing outbox remains the durability boundary. Connectors translate canonical post-policy events to Kafka, HTTP JSON, OTLP/HTTP JSON, Prometheus Remote Write, Splunk HEC, or Datadog Logs. Connector health is reported independently from router process readiness so one backend outage cannot restart or stall healthy delivery lanes.

## Evidence-First Intelligence

The v2 intelligence layer sits above frozen incident evidence; it is not in the ingestion critical path.

```text
Frozen Incident
   +--> Evidence Graph + structured changes
   +--> Incident Replay
   +--> Cost Simulation
             |
             v
Evidence-First Investigator
   +--> cited findings + contradictions
   +--> incident comparison
   `--> advisory recommendation
             |
             v
Human lifecycle review / approval / activation
```

The investigator has no production mutation interface. The v1.6 lifecycle remains the only path to configuration activation.
