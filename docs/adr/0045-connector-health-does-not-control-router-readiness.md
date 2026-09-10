# ADR 0045: Connector Health Does Not Control Router Process Readiness

**Status:** Accepted  
**Date:** 2026-09-10

## Context
Restarting the router because one vendor is unavailable would disturb healthy destination lanes.

## Decision
Router readiness means the process and durable routing store initialized. Connector readiness is probed and reported separately.

## Consequences
A failed backend can accumulate retry/DLQ state while healthy connectors continue delivering.
