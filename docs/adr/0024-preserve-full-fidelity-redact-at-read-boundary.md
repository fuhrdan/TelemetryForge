# ADR 0024: Preserve Full Fidelity, Redact at Read Boundary

**Status:** Accepted  
**Date:** 2026-09-09

## Context

The Incident Flight Recorder exists to preserve evidence, but operators may not
want sensitive tags/payloads exposed to every dashboard/API reader.

## Decision

v1.0 preserves stored evidence and adds configurable presentation-time
redaction for API/SSE responses.

Sensitive configured tag values become `[REDACTED]`.

Payloads can be replaced with a redaction marker.

## Consequences

PostgreSQL/backup/direct-SQL access remains highly sensitive.

Organizations that are legally/policy prohibited from storing specific fields
still need destructive ingestion-time redaction before TelemetryForge accepts
those values; read-time redaction is not a substitute.
