# ADR 0038: Import Archives as Frozen Evidence, Not Production Traffic

**Status:** Accepted  
**Date:** 2026-09-09

## Context

Re-injecting imported incident telemetry into the normal pipeline could trigger
sampling, routing, alerts, duplicate persistence, or vendor costs.

Archived configuration is historical evidence and may be unsafe/stale.

## Decision

Import writes only frozen incident metadata/events and import provenance.

It never:

- calls ingestion;
- populates normal telemetry tables;
- activates policy/shaping/routing snapshots;
- sends destination traffic.

Cross-tenant import requires an explicit flag.

Existing incident IDs are not overwritten.

## Consequences

Imported evidence is immediately available for investigation/replay while the
live system remains unchanged.

Historical replay/cost artifacts stay in the file rather than masquerading as
new live analysis runs.
