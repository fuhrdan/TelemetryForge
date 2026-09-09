# ADR 0018: Make Cost Simulation Assumptions Explicit

**Status:** Accepted  
**Date:** 2026-09-09

## Context

Telemetry cost is vendor- and contract-specific. A generic calculator that
silently assigns dollar prices would create false precision.

## Decision

The simulator always computes policy effects on:

- canonical JSON sample bytes
- exact distinct sample series
- changed events
- linear monthly volume projection

Dollar estimates are omitted unless an operator supplies a strict pricing JSON
file.

Every result stores its assumptions.

## Consequences

Out-of-the-box simulations remain useful for comparing active versus candidate
policy without pretending to know a customer's vendor contract.

Pricing models can be reviewed and versioned separately from application code.
