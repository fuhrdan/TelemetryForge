# ADR 0007: Use PostgreSQL with TimescaleDB

**Status:** Accepted  
**Date:** 2026-09-09

## Context

TelemetryForge needs relational metadata and efficient time-oriented telemetry
queries. Running completely separate database technologies would increase local
and operational complexity early in the project.

## Decision

Use PostgreSQL with the TimescaleDB extension.

Ordinary PostgreSQL tables hold relational/idempotency data. The
`telemetry_events` hypertable holds high-volume timestamped telemetry.

## Alternatives considered

- **Plain PostgreSQL:** simpler, but does not demonstrate or provide the
  time-series partitioning/retention features wanted for this project.
- **Redis as primary storage:** useful for hot state, but not appropriate as the
  durable analytical system of record.
- **ClickHouse:** compelling at very high analytical scale, but adds a separate
  database platform before the project's relational metadata needs are mature.
- **Separate PostgreSQL + time-series database:** valid at larger scale, but
  unnecessary operational complexity for the current milestone.

## Consequences

TelemetryForge can use familiar SQL while gaining hypertables, time-oriented
indexing, and retention policies. The architecture retains a storage interface
so another backend can be introduced later without coupling Kafka/HTTP code to
SQL.
