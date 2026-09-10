# ADR 0042: Rollbacks Are Evidence, Not Causal Proof

**Status:** Accepted  
**Date:** 2026-09-10

## Context

A system can recover after rollback for unrelated reasons: dependency recovery,
traffic movement, cache warmup, or coincident operator action.

## Decision

Model explicit `rollback_of` relationships and post-rollback recovery signals
as evidence associations.

Evidence Graph and Change Intelligence wording must state that sequence does
not prove the rollback caused recovery or that the original change caused the
incident.

## Consequences

Operators get high-value temporal evidence without an unsafe automatic root-
cause claim.
