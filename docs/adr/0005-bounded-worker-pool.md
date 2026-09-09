# ADR 0005: Use a Bounded Worker Pool

**Status:** Accepted  
**Date:** 2026-09-09

## Context

Kafka can retain substantially more telemetry than one worker process can hold
in memory. If processing slows, an unbounded application queue would turn a
downstream outage into a worker out-of-memory failure.

## Decision

TelemetryForge uses a fixed number of Go workers fed by a bounded channel.
Submitting to a full channel blocks until capacity is available or the service
context is cancelled.

## Alternatives considered

- **Unbounded application queue:** simple, but unsafe during sustained backlog.
- **One goroutine per event:** easy to write, but provides no useful concurrency
  ceiling and can overwhelm downstream dependencies.
- **Process directly in the Kafka poll loop:** safe but unnecessarily serial.

## Consequences

Kafka becomes the durable backlog and consumer lag becomes the primary scaling
signal. Worker concurrency and queue capacity are explicit operational knobs.
