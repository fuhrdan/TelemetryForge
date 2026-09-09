# ADR 0002: Version the canonical event envelope

**Status:** Accepted

## Context

TelemetryForge will ingest telemetry from independently deployed producers. Their schemas will evolve at different rates.

## Decision

Every accepted event must contain `schema_version`. Stable routing fields live in a canonical top-level envelope while arbitrary application-specific data can live in `payload`.

## Alternatives considered

- Unversioned JSON: simplest initially but makes compatibility and replay ambiguous.
- Separate schemas for every endpoint: stricter but creates unnecessary fragmentation before the schema registry exists.

## Consequences

Compatibility can be reasoned about explicitly. Later schema-registry and replay capabilities have a stable version boundary from the project's first release.
