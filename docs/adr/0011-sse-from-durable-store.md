# ADR 0011: Feed SSE from Durable Shared Storage

**Status:** Accepted  
**Date:** 2026-09-09

## Context

The dashboard needs live telemetry. An in-memory broadcaster is easy to build
but only knows about events seen by one process, which conflicts with the
project's horizontal-scaling goals.

## Decision

The Go gateway exposes an SSE endpoint that polls newly persisted events from
the shared TimescaleDB store.

## Alternatives considered

- **Gateway in-memory broadcaster:** lowest latency, but incorrect with multiple
  replicas and disconnected from worker persistence.
- **Direct browser Kafka access:** poor security/operational boundary.
- **Redis/pub-sub immediately:** appropriate later but adds another dependency
  before the dashboard traffic justifies it.
- **WebSockets:** unnecessary for a server-to-browser-only stream.

## Consequences

The first implementation is simple and replica-safe but spends one periodic
database query per SSE client. The public stream contract can remain stable
when the internal source is replaced later.
