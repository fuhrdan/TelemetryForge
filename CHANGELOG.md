# Changelog

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
