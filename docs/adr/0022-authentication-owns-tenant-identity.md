# ADR 0022: Authentication Owns Tenant Identity

**Status:** Accepted  
**Date:** 2026-09-09

## Context

Allowing telemetry clients to choose `tenant_id` would make tenant isolation a
client convention rather than an authorization boundary.

## Decision

The gateway derives tenant identity from the authenticated principal.

Client-supplied `tenant_id` is rejected.

Tenant identity is then propagated through Kafka, worker policy state, and
storage.

## Consequences

Ingestion clients need separate credentials for separate tenants.

Internal/CLI operations must create explicit trusted tenant context.

Testing must cover both persistent and in-memory tenant isolation.
