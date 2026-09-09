# ADR 0006: Commit Kafka Offsets After Processing

**Status:** Accepted  
**Date:** 2026-09-09

## Context

Committing an offset before processing finishes can lose telemetry if the
worker crashes after the commit. Committing after successful processing can
occasionally process a record twice if the worker crashes before the commit.

## Decision

Disable Kafka auto-commit and commit records after successful processing.

## Alternatives considered

- **Commit before processing:** lower duplicate risk, but permits data loss.
- **Exactly-once transactions immediately:** adds substantial complexity before
  TelemetryForge has transactional downstream persistence.

## Consequences

TelemetryForge provides at-least-once processing. Downstream operations must be
designed for idempotency, using the canonical event ID where possible.
