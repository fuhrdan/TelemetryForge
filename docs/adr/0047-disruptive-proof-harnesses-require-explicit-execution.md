# ADR 0047: Disruptive Proof Harnesses Require Explicit Execution

**Status:** Accepted  
**Date:** 2026-09-11

## Context

Failure testing stops brokers/databases/workers and can restart Kubernetes
workloads. Merely inspecting tooling must never cause those side effects.

## Decision

Every disruptive harness requires `--execute`. Kubernetes restart additionally
requires `--ack-cluster-change`.

## Consequences

Proof workflows remain executable while accidental invocation is guarded.
Automation must opt into the side effect visibly.
