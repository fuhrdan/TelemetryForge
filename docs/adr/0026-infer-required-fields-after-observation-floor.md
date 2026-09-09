# ADR 0026: Infer Required Fields After an Observation Floor

**Status:** Accepted  
**Date:** 2026-09-09

## Context

A field missing from one event may be an optional field rather than schema
breakage.

Immediately treating all first-seen fields as required would create noisy false
positives.

## Decision

A field becomes inferred required only after:

```text
20 or more observations
95% or greater presence
```

Before that point a missing field is not breaking drift.

An established field changing JSON type remains breaking immediately because
two incompatible types under the same declared version are directly observed.

## Consequences

Schema health becomes useful on sparse event payloads instead of assuming every
field is mandatory.

The 20/95 thresholds are v1.1 code-level defaults and should become versioned
schema policy if operators need per-source tuning later.
