# ADR 0021: Evidence Association, Not Automated Causality

**Status:** Accepted  
**Date:** 2026-09-09

## Context

Incident evidence can reveal relationships, but temporal proximity or shared
metadata does not prove root cause.

## Decision

The Evidence Graph stores typed edges as:

- supporting
- contradicting
- related

Every edge includes a human-readable basis.

Built-in hypotheses summarize evidence status but never label an event as the
root cause.

Graph responses include an explicit non-causality disclaimer.

## Consequences

The graph is useful for investigation while remaining conservative.

A future ML/AI explanation layer must cite graph evidence and preserve
contradictory evidence rather than replacing this model with an opaque causal
score.
