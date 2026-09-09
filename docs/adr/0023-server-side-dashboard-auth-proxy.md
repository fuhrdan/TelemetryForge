# ADR 0023: Server-Side Dashboard Authentication Proxy

**Status:** Accepted  
**Date:** 2026-09-09

## Context

Browser JavaScript should not contain a reusable gateway API key, and
`EventSource` cannot conveniently attach a custom Authorization header.

## Decision

The Next.js server proxies `/telemetry-api/*` to the Go gateway.

The server injects one read-only dashboard API key from its environment.

The browser never receives the raw key.

## Consequences

Dashboard deployment becomes a trusted server component.

The key must remain read-only and tenant-scoped.

Direct static-only hosting of the dashboard is no longer the production
authentication model.
