# ADR 0031: Isolate Destination Retry, DLQ, and Fallback

**Status:** Accepted  
**Date:** 2026-09-09

## Context

One observability backend may fail while another remains healthy.

A global retry queue would recreate the coupling the router is intended to
remove.

## Decision

Each destination has independent:

- concurrency
- retry parameters
- due-delivery claims
- health state
- terminal dead-letter records
- optional failure fallback

Router replicas claim work with `FOR UPDATE SKIP LOCKED` and a lease.

Fallback graphs are validated to prevent cycles.

## Consequences

Backlog and failures remain attributable to one destination.

Delivery is at-least-once rather than exactly-once. Destinations should use the
canonical event ID when implementing deduplication.
