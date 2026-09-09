# ADR 0010: Keep a Short Full-Fidelity Flight Recorder

**Status:** Accepted  
**Date:** 2026-09-09

## Context

Future sampling, filtering, cardinality controls, and retention rules may remove
telemetry that later proves important during an incident.

## Decision

Record a short full-fidelity copy of every decoded event before normalization.
The rolling TimescaleDB buffer retains 30 minutes by default.

Incident windows can be frozen into durable incident storage before the rolling
buffer expires.

## Alternatives considered

- **Rely only on the main telemetry table:** cannot protect evidence from future
  sampling/filtering rules.
- **Keep all raw telemetry forever:** simple but defeats cost-control goals.
- **Object-storage archive immediately:** useful later, but unnecessarily heavy
  for the first local implementation.

## Consequences

TelemetryForge spends additional short-term storage/write capacity to preserve
incident evidence. Later releases can move the rolling buffer to cheaper/local
storage while keeping the same incident-freeze contract.
