# ADR 0049: Use Deterministic, Cited Evidence Synthesis

**Status:** Accepted  
**Date:** 2026-09-11

## Context

The v2 investigation layer must help operators synthesize existing evidence without turning an opaque model answer into operational fact.

## Decision

The built-in v2 investigator is deterministic. It consumes the Evidence Graph plus recorded replay/cost artifacts and emits findings that cite exact graph edges and node IDs.

It exposes contradictory evidence and has an explicit `insufficient_evidence` state.

No external model is required for this default path.

## Alternatives

### Free-form generative root-cause text

Rejected as the default because it weakens reproducibility and evidence provenance.

### No synthesis layer

Rejected because operators still benefit from a bounded summary of the graph that TelemetryForge has already captured.

## Consequences

Repeated runs over equivalent evidence are structurally reproducible. New synthesis rules must be source-controlled and tested.
