# TelemetryForge v0.3.0 Release Notes

## Consumer Groups, Worker Pools & Backpressure

v0.3.0 turns the durable Kafka stream introduced in v0.2.0 into a real
processing pipeline.

### Highlights

- Added a standalone Go worker service.
- Added Kafka consumer group `telemetryforge-processors`.
- Added a configurable, bounded worker pool.
- Disabled Kafka auto-commit and commit records after successful processing.
- Added cooperative group balancing for friendlier horizontal scaling.
- Added the first processor: a small source/type normalizer.
- Added explicit failure behavior and groundwork for poison-event handling.
- Added worker service to Docker Compose.
- Added `make run-worker` and `make kafka-groups`.
- Added human-readable worker, backpressure, and failure documentation.
- Added ADRs for bounded queues and post-processing offset commits.

### Why this release matters

Kafka is now more than a producer destination. It is the durable buffer between
independently scalable ingestion and processing tiers.

When processing slows, the bounded queue stops unlimited memory growth and
allows backlog to remain in Kafka. This creates a measurable scaling signal:
consumer lag.

### Delivery semantics

Processing is at least once. A successfully processed record is explicitly
committed. Failed work remains uncommitted and can be retried. Future
persistence will use event IDs to make side effects idempotent.

### Known limitations

- Poison records do not yet have a dead-letter queue.
- Retry ceilings and exponential backoff arrive in the reliability milestone.
- The lag value in code is a local estimate; broker-derived lag metrics arrive
  with full observability instrumentation.
- No durable processed-event database exists yet.
- Authentication and tenant isolation are not implemented.

These limitations are documented intentionally rather than hidden behind a
"production ready" claim.
