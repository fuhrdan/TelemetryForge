# ADR 0029: Series Budgets Are Advisory, Not Destructive

**Status:** Accepted  
**Date:** 2026-09-09

## Context

A source or tenant can exceed an acceptable unique-series envelope even when no
single label clearly identifies the correct destructive action.

Automatically dropping telemetry when an aggregate budget is crossed could
remove useful evidence without explaining what was lost.

## Decision

Policy documents can define hourly unique-series budgets.

A budget tracks the HLL of full series identities across its matching scope and
reports `healthy`, `warning`, `critical`, or `exceeded`.

Budgets never mutate telemetry in v1.2.0.

Explicit dimension policy continues to own `drop_tag` and `quarantine` actions.

## Consequences

Operators get proactive cost/cardinality pressure visibility without introducing
an implicit sampling/drop policy.

Later Adaptive Sampling can consume this signal through an explicit reviewed
policy change instead of silently changing behavior in v1.2.
