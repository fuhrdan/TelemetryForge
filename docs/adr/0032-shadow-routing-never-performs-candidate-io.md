# ADR 0032: Shadow Routing Never Performs Candidate I/O

**Status:** Accepted  
**Date:** 2026-09-09

## Context

Operators need to understand how a routing change would alter fan-out before
new backends receive production telemetry.

## Decision

The worker evaluates active and candidate routing against the same post-policy
event.

Only the active plan is written to the routing outbox.

The candidate plan is reduced to added/removed destination names and persisted
as a shadow diff.

## Consequences

Candidate routing can be exercised on live production-shaped telemetry without
sending a single event to candidate destinations.

This mirrors the safety philosophy of the existing shadow Cardinality Firewall
while keeping routing policy independent from mutation policy.
