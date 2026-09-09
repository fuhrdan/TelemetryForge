# ADR 0027: Separate Application Schema Version from OpenTelemetry Schema URL

**Status:** Accepted  
**Date:** 2026-09-09

## Context

TelemetryForge already requires `schema_version` from the application.
OpenTelemetry independently defines `schema_url` for the semantic-convention
schema associated with telemetry.

Treating them as the same value would conflate application payload evolution
with OpenTelemetry semantic-convention evolution.

## Decision

Keep both fields:

```text
schema_version  -> application schema registry/history key
schema_url      -> optional OpenTelemetry semantic-convention identifier
```

A `schema_url` change under an unchanged application schema version is recorded
as a warning.

## Consequences

Applications can version their domain payload independently of OpenTelemetry.

TelemetryForge can later use OpenTelemetry schema transformations without
breaking application schema history.
