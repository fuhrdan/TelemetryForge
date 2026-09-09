# Changelog

## [Unreleased]

_No changes yet._

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
