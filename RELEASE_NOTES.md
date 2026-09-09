# TelemetryForge v0.1.0 — Foundation & Ingestion API

Released: 2026-09-09

v0.1.0 establishes the contract and engineering structure for TelemetryForge. It is deliberately small: the release proves the gateway, canonical telemetry envelope, validation model, documentation discipline, tests, container packaging, and CI foundation before distributed infrastructure is introduced.

## Highlights

- High-concurrency Go HTTP gateway based on the standard library.
- Versioned generic telemetry envelope designed for events, logs, webhooks, and metrics.
- Separate domain validation so future HTTP, gRPC, Kafka replay, and CLI paths can share rules.
- Strict unknown-field rejection and a 1 MiB request limit at the ingestion edge.
- Generated UUIDv4-compatible event IDs.
- Structured JSON logging and graceful shutdown.
- Unit and handler tests plus GitHub Actions CI.
- Docker/Compose packaging and a one-command developer check target.
- ADRs and code-documentation standards established before the distributed pipeline exists.

## Next release

v0.2.0 introduces Apache Kafka, durable publishing, topic/partition strategy, producer acknowledgements, correlation propagation, and integration tests while preserving the v0.1 API contract.
