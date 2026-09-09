# ADR 0030: Durable Outbox Separates Routing from Processing

**Status:** Accepted  
**Date:** 2026-09-09

## Context

Sending directly to multiple external backends inside the Kafka processing
worker would couple source acknowledgement to the slowest/least reliable
backend.

## Decision

The primary worker writes routing intents to a PostgreSQL outbox after primary
persistence.

A separate router service performs destination I/O.

Outbox uniqueness is:

```text
tenant_id + event_id + destination
```

so worker retry is idempotent.

## Consequences

A destination outage cannot block the primary Kafka processing partition after
routing intent is durable.

Routing now depends on PostgreSQL availability at planning time. That is an
intentional durability dependency: TelemetryForge does not acknowledge a source
record if its required routing intents cannot be recorded.
