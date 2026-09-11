# ADR 0050: Intelligence Recommendations Never Mutate Production

**Status:** Accepted  
**Date:** 2026-09-11

## Context

TelemetryForge can combine replay and cost evidence into useful candidate-policy advice, but recommendation generation must not bypass the v1.6 lifecycle safety model.

## Decision

Recommendations are advisory objects only. Any recommendation that could affect production sets `requires_human_approval=true` and names the required lifecycle path.

The intelligence package cannot activate or modify production configuration.

## Consequences

Operators retain control. Existing shadow, evidence, approval, control-scope authorization, scheduling, activation, and rollback rules remain authoritative.
