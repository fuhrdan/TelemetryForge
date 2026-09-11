# ADR 0048: Backend Outage Does Not Collapse Unrelated Routing Lanes

**Status:** Accepted  
**Date:** 2026-09-11

## Context

Connector isolation is a core Telemetry Router promise and should be tested as
an operational property rather than inferred from architecture alone.

## Decision

Ship a connector-outage proof fixture that fans the same event to a healthy
Kafka connector and an intentionally unreachable HTTP connector. The proof
checks that the healthy lane delivers while the failed lane retries/dead-letters
and router readiness remains healthy.

## Consequences

Destination isolation has an executable regression scenario and can generate a
reviewable `.tfproof.json` artifact.
