# ADR 0019: Use Prometheus and OTLP for Self-Observability

**Status:** Accepted  
**Date:** 2026-09-09

## Context

TelemetryForge cannot credibly control observability pipelines if its own
backpressure, lag, failures, and request behavior are invisible.

## Decision

Expose process-local Prometheus metrics and emit sampled OpenTelemetry traces
through OTLP.

Prometheus labels must remain bounded. HTTP metrics use registered route
patterns rather than raw paths.

Kafka records carry W3C Trace Context/Baggage so gateway and worker spans can be
connected.

The local Compose stack provisions Prometheus, Grafana, an OpenTelemetry
Collector, and Tempo using pinned versions.

## Consequences

Operators can distinguish gateway saturation, Kafka backlog, worker pressure,
and processing failure.

Metrics endpoints and tracing infrastructure become operational surfaces that
require network/authentication controls in production.
