# ADR 0035: Shadow Shaping and Explicit Visibility Measures

**Status:** Accepted  
**Date:** 2026-09-09

## Context

Sampling changes can reduce cost while accidentally hiding useful incident
signals. A single synthesized score would create false confidence.

## Decision

Evaluate an optional candidate shaping configuration beside the active one but
never apply candidate output to production.

Historical preview reports separate event, byte, protected-event, and
source/type-retention measurements.

Do not publish a single opaque visibility score.

## Consequences

Operators can inspect exactly what a more aggressive candidate would remove and
can combine that result with Incident Replay, Cost Simulation, and Evidence
Graph analysis before promotion.
