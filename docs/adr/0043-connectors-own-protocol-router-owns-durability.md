# ADR 0043: Connectors Own Protocol Translation; Router Owns Durability

**Status:** Accepted  
**Date:** 2026-09-10

## Context
Vendor retry behavior can leak into core processing and couple unrelated destinations.

## Decision
Connectors own protocol translation/readiness and may classify errors. The Telemetry Router retains durable outbox leasing, retries, DLQ, fallback, and completion.

## Consequences
Adapters can change without changing delivery-state semantics, and permanent rejections can skip useless retries.
