# TelemetryForge Roadmap

| Version | Status | Milestone |
|---|---|---|
| v0.1.0 | Released | Gateway foundation and canonical envelope |
| v0.2.0 | Released | Kafka durable publishing |
| v0.3.0 | Released | Consumer groups, bounded workers, backpressure |
| v0.4.0 | Released | PostgreSQL/TimescaleDB persistence |
| v0.5.0 | Released | Retries, DLQ, Flight Recorder |
| v0.6.0 | Released | Dashboard and automatic incident capture |
| v0.7.0 | Folded into v0.8.0 | Reliability/repository hardening |
| v0.8.0 | Released | Kubernetes, Cardinality Firewall, Terraform, policy/shadow pipeline |
| v0.9.0 | Released | Incident Replay, Cost Simulator, self-observability, real Kafka lag, k6 methodology |
| v1.0.0 | Released | Evidence Graph, auth/tenant isolation, redaction, Kafka security, production profile/runbooks |
| v1.1.0 | Delivered development milestone | Schema Intelligence, schema history/drift, OpenTelemetry semantic-convention awareness |
| v1.2.0 | Delivered development milestone | Distributed Cardinality Intelligence, hourly series budgets, forecasting |
| v1.3.0 | Delivered development milestone | Multi-destination Telemetry Router, isolated retry/DLQ/fallback, shadow routing |
| v1.4.0 | Delivered development milestone | Adaptive Sampling & Telemetry Shaping |
| v1.5.0 | Delivered development milestone | Portable `.tfincident` archives, encryption, import provenance, offline investigation |
| v1.6.0 | Delivered development milestone | Policy lifecycle model, approval/evidence gates, managed configuration state foundation |
| v1.7.0 | Delivered development milestone | Change Intelligence, before/after analysis, observed blast radius, rollback evidence |
| v1.8.0 | Delivered development milestone | Connector Platform, OTLP/HTTP JSON, Prometheus Remote Write, Kafka/HTTP/vendor adapters, runtime health |
| v1.9.0 | Delivered development milestone | Scale, HA, chaos, backup/restore and reproducible operational proof artifacts |
| v2.0.0 | Released | Evidence-first investigator, cited incident comparison, human-gated recommendations |
| v2.1.0 | Released | Durable Edge WAL, crash recovery, ordered replay, capacity backpressure |
| v2.2.0 | Released | Replicated durability, quorum acceptance, failure-domain awareness |
| **v2.3.0** | **Released** | **Global routing mesh, deterministic ownership, health-aware failover** |

## v1.2.0 delivered

### Shared cardinality state

- production PostgreSQL/TimescaleDB-backed cardinality tracker
- atomic HLL register merge across worker replicas
- exact first-16 hashed uniques
- tenant/active-shadow/source/type/dimension/hour key
- seven-day shared-state retention
- same hourly semantics in isolated replay/cost trackers

### Cardinality forecasting

- observed unique counts
- one-hour projected unique counts
- unique growth/minute
- top exploding dimensions API/dashboard/CLI

### Series budgets

- versioned policy budgets
- tenant-wide and source-pattern scopes
- full series-identity estimation
- warning / critical / exceeded thresholds
- active/shadow policy budget visibility
- non-destructive budget contract

### Operations

- `telemetryctl cardinality top`
- `telemetryctl cardinality budgets`
- distributed state/budget APIs
- dashboard panels
- integration coverage for shared state across independent tracker instances
- ADRs 0028-0029

## v1.3.0 delivered

### Routing policy

- tenant/source/type/severity/tag matching
- multi-rule fan-out with destination deduplication
- unmatched-event fallback
- strict JSON validation

### Delivery isolation

- durable routing outbox
- separate router service
- per-destination concurrency lanes
- bounded exponential retry
- per-destination DLQ
- optional terminal failure fallback
- leased `FOR UPDATE SKIP LOCKED` claims for multiple router replicas

### Destination types

- Kafka
- HTTP/webhook
- secret-backed HTTP bearer token environment references

### Safety / operations

- shadow routing records added/removed destinations without candidate I/O
- destination health and queue API/dashboard
- routing CLI validation/inspection/DLQ requeue
- router health/metrics endpoint
- Docker Compose and Kubernetes router service
- ADRs 0030-0032

## v1.4.0 delivered

- deterministic and pressure-aware sampling
- protected error/severe/high-latency/audit/deployment/incident telemetry
- tag drop/rename and payload shaping
- fail-open shaping audit path
- active/shadow shaping comparison
- one-hour event/byte retention statistics
- frozen-incident visibility preview
- shaping dashboard/metrics/CLI
- ADRs 0033-0035

## v1.5.0 delivered

- portable `.tfincident` format
- per-member SHA-256 integrity manifest
- exact configuration snapshots
- frozen event / schema / replay / cost / Evidence Graph evidence
- optional AES-256-GCM outer encryption
- offline inspect/verify/report commands
- tenant-aware import with explicit remap
- no-overwrite semantics
- durable import provenance
- archive import history API/dashboard
- ADRs 0036-0039

## v1.6.0 delivered foundation

- immutable policy/shaping/routing artifact model
- draft/shadow/approved/scheduled/active/retired states
- creator self-approval protection
- approval and evidence activation gates
- explicit rollback to retired immutable versions
- dedicated global `control` authorization scope
- runtime-load/convergence storage model
- migration 013

## v1.7.0 delivered

- authenticated structured change ingestion
- tenant-scoped deployment/rollback/release/configuration marker store
- Git SHA/build/environment/version context
- conservative before/after error-rate and p95 analysis
- observed source blast-radius estimate
- rollback/recovery evidence
- persisted analysis snapshots
- API, CLI, dashboard, integration coverage, and demo workflow
- Evidence Graph structured change/rollback relationships
- `.tfincident` change marker snapshots
- corrected shaping-before-cardinality worker ordering
- ADRs 0040-0042

## v1.8.0 delivered

- stable connector contract and capability catalog
- Kafka, generic HTTP JSON, OTLP/HTTP JSON, Prometheus Remote Write, Splunk HEC, Datadog Logs
- OTLP/HTTP JSON log and numeric gauge/sum ingestion
- environment-backed connector credentials
- connector runtime readiness/capability heartbeats
- connector catalog/runtime APIs, dashboard, metric, and CLI
- migration 015 and ADRs 0043-0045

## v1.9.0 delivered

- versioned `.tfproof.json` operational evidence
- strict pass/fail/assertion/measurement validation
- whole-file and raw-evidence SHA-256 provenance
- proof recording/list API/dashboard
- k6 benchmark result capture
- Compose preflight and Kafka outage/recovery proof
- PostgreSQL outage/recovery persistence proof
- connector outage isolation proof
- multi-worker crash/failover persistence proof
- real TimescaleDB dump/restore verification
- explicit Kubernetes rolling-restart proof
- manual GitHub Actions proof workflow
- zero-unavailable gateway/router/dashboard rolling defaults
- bounded worker rolling unavailability
- topology spread, router PDB, router HPA
- Compose graceful-stop periods
- migration 016
- ADRs 0046-0048

## v2.0.0 delivered

- deterministic Evidence-First Investigator
- exact Evidence Graph edge/node citations
- explicit insufficient/mixed/supported states
- contradictory-evidence visibility
- tenant-scoped investigation history
- reproducible incident comparison with cited matching nodes
- recorded Cost Simulation / Replay-backed policy evaluation recommendations
- mandatory human lifecycle approval for production changes
- API, CLI, dashboard, migration 017, tests and ADRs 0049-0051


## v2.1.0 delivered

- `telemetryforge-edge` service with fsync-before-acceptance segmented WAL
- CRC32 + event SHA-256 integrity
- edge/source sequence continuity
- atomic checkpoint and ordered Kafka replay
- capacity backpressure and segment compaction
- torn-tail recovery / completed-frame fail-closed behavior
- Docker Compose persistence, Kubernetes StatefulSet/PVC
- status API, tests, proof harness, docs, ADR 0052


## v2.3.0 delivered

### Global route ownership

- authenticated peer topology advertisements
- active health probing and stale-state rejection
- cloud/region/zone failure-domain metadata
- rendezvous-hash ownership keyed by tenant/source
- `locality` and active-active `global` routing modes

### Adaptive eligibility and failover

- exclude draining nodes
- exclude unhealthy/stale peers
- exclude nodes above configured WAL-pressure threshold
- ordered candidate failover when a downstream path fails
- peer identity validation on health and forward operations

### Safe forwarding boundary

- origin WAL/quorum remains the durability authority
- remote forwards terminate at the selected node's local Kafka publisher
- no recursive mesh forwarding
- origin commits only after downstream success
- explicit at-least-once behavior for ambiguous network failures

### Operations and evidence

- mesh snapshot in `/edge/status`
- deterministic `/edge/route?key=...` inspection
- active-active three-edge Compose example
- Kubernetes configuration/secrets surface
- `proof/mesh-failover.sh` operational proof harness
- ADR 0054

## v2.2.0 delivered

- synchronous edge-to-edge durable replication
- crash-safe receiver replica logs
- idempotent origin-sequence retry and conflict rejection
- local/regional/cross-region/cross-cloud durability classes
- configurable quorum including local durable copy
- zone/region/cloud failure-domain enforcement
- quorum-aware readiness
- replication retry before downstream Kafka replay
- bearer-token protected internal replication API
- bounded replica retention with release-through checkpoints and compaction
- three-edge Compose regional quorum topology
- Kubernetes replica-storage configuration hooks
- tests, proof harness, docs, ADR 0053
