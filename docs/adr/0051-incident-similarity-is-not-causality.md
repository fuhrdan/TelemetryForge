# ADR 0051: Incident Similarity Is Not Causality

**Status:** Accepted  
**Date:** 2026-09-11

## Context

Two incidents can share services, event types, hypotheses, or deployment metadata while having different underlying causes.

## Decision

Use a transparent weighted Jaccard comparison over captured Evidence Graph features. Return overlap status, component notes, and cited node IDs. Always state that similarity does not imply shared root cause.

## Consequences

Incident comparison is reproducible and useful for navigation without being promoted into an automated causal claim.
