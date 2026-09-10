# ADR 0040: Normalize Change Markers Before Lossy Shaping

**Status:** Accepted  
**Date:** 2026-09-10

## Context

Deployments and rollbacks are disproportionately valuable during incident
analysis. Sampling them out would destroy evidence needed to interpret later
telemetry.

## Decision

Record structured change markers after Flight Recorder, normalization, and
Schema Intelligence, but before Adaptive Shaping.

Protect the operational change event vocabulary in the active shaping policy.

The canonical event continues through the ordinary pipeline after marker
recording.

## Alternatives

### Derive changes only from persisted shaped telemetry

Rejected because a future shaping misconfiguration could erase the exact
change marker needed to diagnose an incident.

### Direct-write changes from the gateway

Rejected because that would split durability semantics between Kafka and
PostgreSQL.

## Consequences

Change evidence follows the durable Kafka path while being captured before any
lossy shaping stage.
