# ADR 0039: Offline Incident Report Is Self-Contained and Script-Free

**Status:** Accepted  
**Date:** 2026-09-09

## Context

Operators may need to inspect an incident on a machine without TelemetryForge,
Kafka, PostgreSQL, Internet access, or trusted browser extensions.

## Decision

Generate a standalone HTML report using Go `html/template`.

The report contains:

- incident metadata;
- Evidence Graph hypotheses/evidence;
- a bounded frozen timeline;
- configuration-snapshot inventory.

It contains no JavaScript, external assets, analytics, or network requests.

All telemetry-controlled text is HTML escaped.

## Consequences

Reports are portable and have a small attack/availability surface.

They remain sensitive evidence files and must be protected like the archive
itself.
