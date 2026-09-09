# ADR 0001: Use Go for the ingestion gateway

**Status:** Accepted

## Context

TelemetryForge requires a small, high-concurrency ingestion service that is easy to deploy in containers and Kubernetes and straightforward to reason about under load.

## Decision

Use Go for the gateway and initial worker services.

## Alternatives considered

- Rust: excellent performance and safety, but higher implementation complexity for this portfolio's first service.
- Node.js: productive for HTTP workloads, but less aligned with the project's explicit systems-engineering goals.
- Python: excellent for analytics and experimentation, but not preferred for the primary high-throughput gateway.

## Consequences

The gateway can use goroutines and the standard HTTP stack with minimal runtime dependencies. Future workers can share domain packages with ingestion services.
