# Changelog

## [Unreleased]

_No changes yet._

## [3.0.0] - 2026-09-14

### Added
- Global Edge Fabric runtime contract with explicit acceptance, delivery, and audit readiness dimensions.
- `GET /edge/fabric/status` with per-capability state for durability, routing, lineage, fast-path replay, and eBPF collection.
- Dependency-free `telemetryforge-fabriccheck` covering healthy, downstream-partition, quorum-loss, WAL-capacity, and required-eBPF scenarios.
- Global Edge Fabric `.tfproof.json` scenario, ADR 0061, v3 architecture documentation, and v3 upgrade/rollback guidance.

### Changed
- Product/service/dashboard/Kubernetes versions advanced to v3.0.0.
- CI now validates the v3 fabric contract and targets the `v3.0.0` release branch.
- v3 runtime status distinguishes backlog-producing downstream degradation from loss of the durable acceptance boundary.

### Compatibility
- Preserved WAL v1/v2 read compatibility, `/api/v1`, `/internal/v1`, `.tfincident`, and `.tfproof.json` formats.
- No database migration is required solely for the Global Edge Fabric contract.

## [2.9.0] - 2026-09-14

### Added
- Deterministic AWS/GCP/Azure/bare-metal multi-cloud failure-domain checker with explicit event accounting.
- Named cloud-outage, region-partition, capacity-pressure, ambiguous-network, and cascading-outage schedules.
- `telemetryforge-multicloudcheck`, `multi-cloud-proof` and destructive logical-cloud Compose proof scenarios.
- GCP GKE and Azure AKS Terraform foundations alongside the existing AWS EKS foundation.
- ADR 0060, multi-cloud proof documentation, CI coverage, and `.tfproof.json` evidence integration.

### Changed
- Product/service versions advanced to v2.9.0.
- Infrastructure validation now covers AWS, GCP, and Azure Terraform foundations.

## [2.8.0] - 2026-09-14

### Added
- Optional Linux tracepoint eBPF collector for aggregate process-exec, socket-connect, and TCP-retransmit counts.
- Raw dependency-free BPF map/program loader for Linux amd64/arm64, with kernel-verifier logging on load failure.
- Durable collector handoff that distinguishes local WAL persistence from later quorum acknowledgement.
- `/edge/ebpf/status`, `telemetryforge-ebpfcheck`, proof scenario, CI job, docs, ADR 0059, and opt-in Kubernetes eBPF overlay.

### Changed
- Edge publisher can return a local-persistence receipt without changing the existing `Publish` contract.
- Product/service versions advanced to v2.8.0.

## [2.7.0] - 2026-09-13

### Added
- Shadow-first worker pressure forecasting with bounded `off`/`shadow`/`auto` modes.
- Audited temporary non-protected sampling multiplier and automatic rollback guardrails.
- Retry-stable autonomy multiplier capture in shaping decisions.
- `/autonomy/status`, `autonomycheck`, Wasm plugin validation, proof scenario, docs, and ADR 0058.

### Changed
- Product/service versions advanced to v2.7.0.
- CI and Operational Proof now validate autonomous-control and Wasm-boundary behavior.

## [2.6.0] - 2026-09-13

### Added
- Bounded replay batching with configurable `TELEMETRYFORGE_EDGE_REPLAY_BATCH_SIZE`.
- Optional per-item-result stream batch publishing contract.
- Asynchronous Kafka batch produce with bounded pooled JSON buffers.
- Bounded reusable WAL scan buffers and fixed frame headers.
- Lock-free bounded MPMC ring primitive with saturation/no-overwrite tests.
- Fast-path diagnostics, allocation-aware benchmarks, CI artifact capture, and Operational Proof scenario.
- ADR 0057 and v2.6 performance/roadmap documentation.

### Changed
- Edge replay checkpoints only the contiguous successful prefix of a batch.
- Product/service versions advanced to v2.6.0.

## [2.5.0] - 2026-09-12

### Formal Verification
- Added TLA+ specifications and finite TLC model configurations for durable ingest, replicated durability, mesh failover, and cryptographic lineage.
- Added a dependency-free Go bounded state-space checker with machine-readable reports.
- Added adversarial protocol action-schedule fuzzing.
- Added SANY/TLC formal-verification CI using a pinned TLA+ tools release.
- Added a formal-verification operational proof scenario and `.tfproof.json` evidence integration.
- Added ADR 0056, formal verification documentation, and explicit safety/liveness claim boundaries.
- Product/service versions advanced to v2.5.0.

## [2.4.0] - 2026-09-12

### Cryptographic Lineage
- Added WAL record format v2 with chained SHA-256 record lineage while retaining v1 read compatibility.
- Added deterministic SHA-256 Merkle roots for closed WAL segments.
- Added Ed25519-signed `.tfseal` attestations chained by previous segment root.
- Added persistent signer identity, explicit production key paths, and fail-closed signer mismatch.
- Retained signed seals after normal WAL compaction so historical segment commitments survive payload cleanup.
- Added `telemetryctl audit keygen` and `telemetryctl audit verify`, including optional trusted-key authentication.
- Added lineage state to edge status, ADR 0055, tests, documentation, and cryptographic-lineage proof harness.
- Product/service versions advanced to v2.4.0.

## [2.3.0] - 2026-09-12

### Added
- Global edge/relay routing mesh with authenticated health advertisements and terminal forwarding.
- Deterministic rendezvous-hash ownership with `locality` and `global` policies.
- WAL-pressure, stale-peer, downstream-readiness, and planned-drain route eligibility.
- Automatic candidate failover while retaining the origin WAL until delivery succeeds.
- `/edge/route` route inspection and mesh topology in `/edge/status`.
- Three-edge active-active Compose example, Kubernetes mesh settings, ADR 0054, and mesh proof harness.

### Changed
- Edge readiness now includes downstream mesh route availability in addition to WAL and replication quorum readiness.
- Product/service versions advanced to v2.3.0.

## [2.2.0] - 2026-09-12

### Replicated Durability
- Added synchronous edge-to-edge durable replication with fsynced receiver replica logs.
- Added local, regional, cross-region, and cross-cloud durability modes.
- Added quorum plus zone/region/cloud failure-domain evaluation before successful non-local acceptance.
- Added idempotent replica retry and conflicting origin-sequence rejection.
- Added quorum-aware readiness and replication-before-Kafka replay.
- Added bearer-token protected internal replication endpoints and replica status reporting.
- Added bounded replica capacity, durable release-through checkpoints, and safe compaction after downstream commit.
- Added three-edge Docker Compose topology, Kubernetes configuration hooks, tests, proof harness, docs, and ADR 0053.

## [2.1.0] - 2026-09-12

### Durable Edge
- Added `telemetryforge-edge` with fsync-before-acceptance local WAL semantics.
- Added segmented crash recovery, CRC32 frame checks, event SHA-256 verification, and fail-closed completed-frame corruption handling.
- Added global edge and per-tenant/source sequences with restart/compaction continuity.
- Added ordered asynchronous Kafka replay, atomic delivery checkpointing, bounded retry, and WAL-capacity backpressure.
- Added `/edge/status`, Docker/Kubernetes persistence, CI coverage, proof harness, documentation, and ADR 0052.

## [2.0.0] - 2026-09-11

### Evidence-First Intelligence
- Added deterministic incident synthesis with exact Evidence Graph citations and contradiction visibility.
- Added explicit insufficient, mixed, and evidence-supported result states.
- Added reproducible incident comparison with cited node IDs and non-causal similarity disclaimer.
- Added advisory shadow-policy recommendations backed by stored cost-simulation and replay artifacts.
- Kept production mutation exclusively behind existing human-reviewed lifecycle promotion.
- Added tenant-scoped intelligence snapshot storage, API, CLI, dashboard, migration 017, docs and ADRs 0049-0051.

## [1.9.0-dev] - 2026-09-10

### Scale, HA & Operational Proof
- Added versioned `.tfproof.json` evidence with strict pass/assertion/measurement/hash validation.
- Added proof verification/record/list CLI, migration 016, proof history API, and dashboard panel.
- Added k6 summary-to-proof capture for smoke/sustained/backpressure workloads.
- Added Compose preflight, Kafka outage/recovery, and two-worker crash/failover proof harnesses.
- Added real TimescaleDB `pg_dump`/restore verification with selected row-count equality checks.
- Added explicitly acknowledged Kubernetes rolling-restart proof.
- Added manual GitHub Actions Operational Proof workflow with result/raw-evidence artifact upload.
- Hardened Kubernetes rolling strategies, topology spread, router PDB, and router HPA.
- Added explicit Compose graceful-stop periods.
- Added ADRs 0046-0048 and HA/chaos/DR/rolling-upgrade/operational-proof documentation.
- Deliberately publish no unmeasured throughput/RPO/RTO/recovery claims.

## [1.8.0-dev] - 2026-09-10

### Connector Platform
- Added stable connector SDK and capability catalog.
- Added Kafka, HTTP JSON, OTLP/HTTP JSON, Prometheus Remote Write, Splunk HEC, and Datadog Logs adapters.
- Added OTLP/HTTP JSON logs/metric ingestion.
- Added typed permanent/transient delivery classification and legacy routing translation.
- Added connector runtime health persistence/API/dashboard/metrics/CLI.
- Added migration 015 and connector documentation/ADRs.

## [1.7.0-dev] - 2026-09-10

### Change Intelligence
- Added authenticated structured change ingestion for deployments, rollbacks, releases, feature flags, configuration, and infrastructure changes.
- Added tenant-scoped normalized change markers with version/Git/build/environment/rollback metadata.
- Added conservative before/after error-rate and p95 analysis with observed blast-radius estimates.
- Added rollback/recovery evidence and persisted analysis snapshots.
- Added change list/analysis/incident-change APIs and telemetryctl change commands.
- Enriched Evidence Graph with structured change, explicit rollback, and recovery-after-rollback relationships.
- Extended `.tfincident` format v1 with optional structured change markers.
- Added dashboard Change Intelligence view and `make demo-change`.
- Corrected worker order so Adaptive Shaping runs before Cardinality Firewall while full evidence/schema/change capture remains before shaping.
- Added migration 014, integration coverage, ADRs 0040-0042, and operator documentation.

## [1.5.0-dev] - 2026-09-09

### Portable Incident Archive
- Added versioned `.tfincident` ZIP/JSON/JSONL format with per-member SHA-256 integrity manifest.
- Added frozen event capture timestamps, Evidence Graph, replay/cost/schema evidence, and exact configuration snapshots.
- Added bounded archive parsing with duplicate/member/path traversal/count validation.
- Added optional AES-256-GCM outer encryption using explicit 256-bit key files.
- Added `telemetryctl incident export`, `verify`, `inspect`, `report`, and `import`.
- Added safe tenant remap, no-overwrite import, and durable import provenance.
- Added archive-import history API/dashboard panel.
- Added standalone script-free offline HTML incident reports.
- Added migration 012, integration coverage, ADRs 0036-0039, and archive/security/runbook documentation.

## [1.4.0-dev] - 2026-09-09

### Adaptive Sampling & Telemetry Shaping
- Added deterministic event-hash sampling and worker-pressure rate reduction.
- Protected error/severe/high-latency/audit/deployment/incident telemetry by default.
- Added tag drop/rename and oversized-payload shaping.
- Added fail-open shaping audit behavior and successful sampled-out early stop.
- Added active/shadow shaping evidence, statistics, metrics, dashboard, CLI preview/pruning, and ADRs 0033-0035.

## [1.3.0-dev] - 2026-09-09

### Telemetry Router
- Added strict active/shadow routing JSON with tenant/source/type/severity/tag matching.
- Added durable routing outbox and separate router service.
- Added Kafka and HTTP/webhook destination types.
- Added per-destination concurrency, retry, health, DLQ, and failure fallback.
- Added leased `FOR UPDATE SKIP LOCKED` claims for multi-replica dispatch.
- Added non-destructive shadow routing with destination-set diffs.
- Added routing API, CLI, dashboard, Docker Compose, Kubernetes, metrics, migration, tests, and ADRs 0030-0032.


## [1.2.0] - 2026-09-09

### Distributed Cardinality Intelligence
- Replaced production replica-local cardinality decisions with shared hourly TimescaleDB/PostgreSQL state.
- Added atomic 64-register HLL merging across worker replicas and exact first-16 hashed unique values.
- Kept replay/cost analysis on isolated local hourly trackers so historical analysis cannot mutate production state.
- Added seven-day retention for shared cardinality windows.

### Series budgets / forecasting
- Added versioned hourly unique-series budgets to policy-as-code.
- Added tenant-wide and source-pattern budget aggregation using full series identities.
- Added healthy/warning/critical/exceeded budget status without destructive automatic action.
- Added observed/projected uniques and unique-growth-per-minute state.

### API / CLI / dashboard
- Added `GET /api/v1/cardinality/state` and `GET /api/v1/cardinality/budgets`.
- Added `telemetryctl cardinality top` and `telemetryctl cardinality budgets`.
- Added Distributed Cardinality and Series Budgets dashboard panels.
- Updated policy validation to include budget configuration.

### Documentation / testing
- Added ADRs 0028-0029.
- Added distributed-cardinality and budget guides.
- Added shared-state integration tests and budget/policy unit tests.


## [1.1.0] - 2026-09-09

### Schema Intelligence
- Added tenant-scoped source/type/version schema registry and history.
- Added optional OpenTelemetry `schema_url` to the canonical event envelope.
- Added bounded payload/tag field discovery and value-free schema fingerprints.
- Capped each accumulated schema registry at 2,048 unique field paths with a deduplicated safety warning.
- Added idempotent schema observations keyed by tenant/event ID.
- Added guarded schema observation ledger pruning with a 35-day default horizon.
- Added additive, type-change, required-field-missing, semantic-convention, and schema-URL drift findings.
- Added 20-observation / 95%-presence required-field inference.
- Added version-to-version compatibility diffs.

### OpenTelemetry semantic conventions
- Added a focused compatibility catalog aligned with semantic conventions 1.44.0.
- Added warnings for legacy HTTP/deployment/network attribute names.
- Added payload semantic-type mismatch detection.

### Operations / UI
- Added fail-open schema registration by default with optional fail-closed mode.
- Added `telemetryctl schema inspect` and `telemetryctl schema diff`.
- Added schema registry/history/drift/diff HTTP APIs.
- Added dashboard Schema Health and schema-history panels.
- Added `make demo-schema` deterministic drift demonstration.
- Added ADRs 0025-0027 and human-readable Schema Intelligence documentation.


## [1.0.0] - 2026-09-09

### Evidence Graph
- Added evidence-backed incident graphs with supporting, contradicting, and contextual relationships.
- Added correlation/trace/source/change/latency/error/recovery relationships.
- Added conservative built-in hypotheses and explicit non-causality disclaimer.
- Added Evidence Graph API, CLI command, dashboard investigation surface, and persistent snapshots.

### Authentication / tenant isolation
- Added hash-only API-key authentication with ingest/read/admin scopes.
- Derived tenant identity from authenticated principal; rejected client-supplied tenant IDs.
- Added tenant identity to the canonical envelope/Kafka key/header.
- Tenant-scoped telemetry, dedup, Flight Recorder, incidents, policy evidence, replay, cost, and graph storage.
- Tenant-isolated Cardinality Firewall and automatic-incident in-memory state.
- Added same-event-ID multi-tenant integration coverage.

### Security
- Added server-side Next.js dashboard auth proxy.
- Added presentation-time tag/payload redaction.
- Added Kafka TLS, custom CA, mTLS, SASL PLAIN, SCRAM-SHA-256, and SCRAM-SHA-512.
- Added production Kubernetes auth/redaction/Kafka-security overlay.
- Added non-root/seccomp/service-account-token hardening.

### Operations / documentation
- Added production deployment profile.
- Added v1 upgrade and rollback runbooks.
- Added v1 portfolio demo scenario.
- Added ADRs 0021-0024 and dedicated authentication/tenant/redaction/dashboard-proxy docs.


## [0.9.0] - 2026-09-09

### Incident Replay
- Added isolated replay of frozen incident evidence through normalization and active/shadow policy.
- Added event-time policy evaluation so replay reflects the captured incident timeline.
- Added compact replay run/event-result persistence and dashboard/API history.
- Added optional output restricted to the `telemetry.replay` namespace.
- Added replay topic-isolation tests at the library boundary.

### Telemetry Cost Simulator
- Added baseline/active/shadow byte and exact sample-series comparison.
- Added linear 30-day canonical-volume projection.
- Added explicit optional pricing-model input; no dollar output without supplied prices.
- Added stored simulation assumptions and dashboard/API history.

### Cardinality
- Added a fixed 16-hash exact counting window before the bounded 64-register HyperLogLog estimator.
- Preserved SHA-256-derived, bounded state without storing raw cardinality values.

### Self-observability
- Added Prometheus gateway/worker metrics with bounded label sets.
- Added sampled OpenTelemetry traces for HTTP, Kafka publish, worker processing, and worker pipeline stages.
- Added W3C Trace Context/Baggage propagation through Kafka headers.
- Added broker-derived Kafka consumer lag via franz-go admin APIs.
- Added worker/gateway `/metrics` endpoints and Kubernetes scrape annotations.

### Local observability stack
- Added pinned OpenTelemetry Collector, Prometheus, Grafana, and Tempo services.
- Added provisioned Prometheus/Tempo Grafana datasources.
- Added a TelemetryForge self-observability dashboard.

### Load testing
- Added pinned k6 smoke, sustained-ingestion, and backpressure scenarios.
- Added benchmark methodology and explicit rules against unsupported throughput claims.

### Documentation / CI
- Added replay, cost, tracing, metrics, self-observability, and benchmark guides.
- Added ADRs 0017-0020.
- Added observability configuration and k6 scenario validation to CI.
- Updated the dashboard with replay and cost-analysis history.


## [0.8.0] - 2026-09-09


### Cardinality Firewall
- Added bounded 64-register HyperLogLog cardinality estimation.
- Added identifier-shaped dangerous-dimension heuristics.
- Added `allow`, `drop_tag`, and `quarantine` policy actions.
- Added SHA-256-derived fingerprints instead of raw high-cardinality values.
- Added finding/diff rate limiting and bounded tracker state.
- Added idempotent quarantine evidence storage keyed by canonical event ID.
- Added Cardinality Firewall API and dashboard panel.
- Added cardinality demo traffic.

### Policy / shadow pipeline
- Added strictly validated versioned JSON policy-as-code.
- Added active and candidate shadow policy files.
- Added candidate-policy comparison without active-path mutation.
- Added policy-difference storage/API/dashboard.
- Added `telemetryctl policy validate`.

### Kubernetes
- Added Kustomize deployment base for gateway, worker, and dashboard.
- Added worker HTTP liveness/readiness service.
- Added HPA, PodDisruptionBudget, ConfigMap, policy ConfigMap, Secret template, and Ingress example.
- Documented worker scaling against Kafka partition count.

### Terraform
- Added Terraform 1.16.2 / AWS provider 6.62.0 EKS foundation.
- Added VPC, public/private subnets, NAT, IAM, EKS cluster, and managed node group.
- Defaulted EKS to Kubernetes 1.36, the newest current EKS standard-support minor.
- Added Terraform CI format/init/validate checks.

### Documentation / repository readiness
- Added root architecture and roadmap documents.
- Added a documentation index, configuration reference, troubleshooting guide, testing guide, release process, and GitHub repository conventions.
- Added automated documentation/link/ADR hygiene checks.
- Added structured GitHub bug/feature forms and a pull-request template.
- Added Dependabot configuration for Go, npm, GitHub Actions, and Docker.

### Reliability audit
- Fixed same-partition out-of-order Kafka commit risk by coordinating commits through the contiguous completed prefix.
- Blocked consumer-group rebalances while bounded polled batches are still processing.
- Added worker batch-completion callbacks and rebalance-safe consumer shutdown.
- Replaced stale v0.2-era Kafka documentation with current delivery/DLQ/partition semantics.
- Added ADR 0013 for concurrent acknowledgement and rebalance coordination.
- Added an ingestion-time index for SSE polling and a deduplication-maintenance index.
- Bounded automatic-incident detector state and release failed freeze cooldown reservations.
- Deduplicated canonical event IDs when freezing/reading incident timelines.

### Development baseline
- Started the untagged v0.7.0 development branch from the immutable v0.6.0 release.
- Updated Go to 1.27.1 and the backend image to Alpine 3.24.
- Updated the dashboard runtime to Node.js 24.21 LTS and React 19.2.8.
- Refreshed GitHub Actions to the current v7 action generation.
- Kept TypeScript pinned to 5.9.3 because TypeScript 7.0.2's native package is not yet a drop-in replacement for the JavaScript compiler API used by the stable Next.js build path.


## [0.6.0] - 2026-09-09

### Added
- Next.js/TypeScript real-time dashboard.
- Server-Sent Events live telemetry endpoint.
- Dashboard summary aggregation API.
- Incident list and captured-event detail APIs.
- Responsive metric cards, SVG signal chart, live stream, and incident timeline.
- Automatic latency-threshold incident detection.
- Automatic per-source error-burst detection.
- Incident cooldown behavior.
- Trigger-reason metadata.
- Dashboard container and Compose service.
- Human-readable dashboard, SSE, and automatic-capture documentation.
- ADR 0011 for durable-store-backed SSE.
- ADR 0012 for explicit automatic incident thresholds.

### Changed
- Worker pipeline now evaluates incident rules after durable persistence.
- Metrics filtering is performed in the storage query rather than after fetching.
- Incident metadata now includes status, trigger reason, and detection time.

### Reliability
- Automatic incident-capture failure is logged without retrying an already
  persisted telemetry event.
- SSE reads shared storage so live visibility is not tied to one gateway process.

## [0.5.0] - 2026-09-09

### Added
- Transient/permanent processing failure classification.
- Bounded exponential retry with jitter.
- Kafka `telemetry.dlq` dead-letter topic.
- Detailed DLQ envelope preserving original source metadata.
- Dead-letter handling for malformed Kafka payloads.
- `telemetryctl` operations CLI.
- Dead-letter event replay.
- 30-minute rolling Incident Flight Recorder.
- Durable incident freeze storage.
- Deduplication pruning lifecycle and CLI.
- Explicit idempotent migration service for existing database volumes.
- Retry, terminal-failure, and Flight Recorder tests.
- Human-readable reliability documentation.
- ADR 0009 for classified retries and DLQ handling.
- ADR 0010 for the Flight Recorder buffer.

### Changed
- Worker pipeline now records full-fidelity telemetry before normalization.
- Terminal failures are acknowledged only after Kafka accepts their DLQ record.
- Local database migrations now run for both clean installations and upgrades.

### Reliability
- Transient dependency failures retry locally with bounded exponential backoff.
- Permanent failures skip wasteful retries.
- Exhausted failures no longer wedge a Kafka partition indefinitely.
- Deduplication cleanup is explicit and guarded by a minimum retention horizon.

## [0.4.0] - 2026-09-09

### Added
- PostgreSQL/TimescaleDB storage layer.
- TimescaleDB telemetry hypertable and SQL migration.
- Transactional event-ID deduplication.
- pgx connection pooling.
- Persistence processor chained after normalization.
- Bounded stored-event query endpoints.
- Source/type/correlation/time and JSONB tag indexes.
- 30-day development retention policy.
- Human-readable storage schema, retention, and query documentation.
- ADR 0007: PostgreSQL with TimescaleDB.
- ADR 0008: dedicated event-ID deduplication table.

### Changed
- Kafka offsets are now committed after durable database persistence succeeds.
- Gateway now exposes read APIs backed by stored telemetry.
- Docker Compose now provisions Kafka and TimescaleDB.

### Reliability
- Duplicate event IDs are safe no-ops at the persistence boundary.
- Database failures leave Kafka records uncommitted for retry.

## [0.3.0] - 2026-09-09

### Added
- Kafka consumer group processing service.
- Bounded Go worker pool with configurable concurrency and queue capacity.
- Manual post-processing offset acknowledgement.
- Cooperative consumer-group balancing.
- Initial normalization processor.
- Docker Compose worker service.
- `make run-worker` and consumer-group inspection target.
- Worker-pool unit tests.
- Human-readable worker model, backpressure, and failure documentation.
- ADR 0005 for bounded worker pools.
- ADR 0006 for manual offset commits.

### Changed
- TelemetryForge now has independently scalable ingestion and processing tiers.

### Reliability
- Failed processing does not acknowledge the Kafka record.
- Queue capacity is bounded so sustained downstream slowness produces Kafka
  backlog instead of unbounded process-memory growth.

All notable changes to TelemetryForge are documented here.

## [0.2.0] - 2026-09-09

### Added
- Kafka-backed streaming Publisher using franz-go.
- Apache Kafka 4.3.1 local KRaft environment.
- `telemetry.raw` and `telemetry.metrics` topics with six development partitions.
- Source-keyed Kafka partitioning.
- Kafka event headers for event ID, schema version, and correlation ID.
- Kafka-aware `/ready` endpoint.
- Broker-backed integration-test target.
- CI Kafka integration job.
- Kafka topic, partitioning, and delivery-semantics documentation.
- ADR 0003 for Kafka selection.
- ADR 0004 for source-based partitioning.

### Changed
- HTTP 202 now requires successful Kafka acknowledgement.
- Publishing failures return HTTP 503 instead of being logged as accepted telemetry.
- Gateway startup now creates a Kafka publisher from environment configuration.
- Local Docker Compose now starts Kafka and initializes topics before the gateway.

### Documentation
- Added event-flow sequence documentation.
- Expanded local-development guide for Kafka workflows.
- Updated README architecture and roadmap status.

## [0.1.0] - 2026-09-09

### Added
- Stateless Go ingestion gateway.
- Canonical versioned telemetry envelope.
- `/health`, `/ready`, `/api/v1/events`, and `/api/v1/metrics` routes.
- Strict JSON validation and 1 MiB request limit.
- Generated event IDs and structured logging.
- Graceful shutdown.
- Initial unit/API tests, Docker build, CI, ADR framework, and project documentation.
