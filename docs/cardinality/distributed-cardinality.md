# Distributed Cardinality Intelligence

TelemetryForge v1.2.0 moves production cardinality decisions from worker-local
memory into a shared, tenant-aware hourly state stored in PostgreSQL/TimescaleDB.

## Why this changed

Before v1.2.0, two worker replicas could each see half of a high-cardinality
stream and independently conclude that the dimension was still below policy.
That made the firewall useful as a local warning mechanism but not a reliable
cluster-wide control.

v1.2.0 makes the database-backed estimator the production source of truth.

## State key

Each active/shadow dimension is tracked by:

```text
tenant_id
mode
source
event_type
dimension
hourly window_start
```

The state contains:

- 64 HyperLogLog registers;
- up to 16 SHA-256-derived hashes for exact low-cardinality counting;
- exact-overflow marker;
- first/last observation timestamps; and
- sample count.

Raw tag values are never stored in this table.

## Atomic replica merging

Each observation hashes one tag value and maps it to one HLL register/rank.

The SQL upsert atomically applies:

```text
register[index] = max(existing, incoming_rank)
```

Because HLL register merge is a component-wise maximum, observations from
multiple workers commute safely into the same shared estimate.

The first 16 unique hashes are also merged under the same row lock. That keeps
small thresholds deterministic while preserving a fixed upper bound.

## Hourly windows

Dimension thresholds in v1.2.0 are evaluated inside the current one-hour UTC
window.

This gives the policy an operational meaning such as:

> request_id is projected to create more than 100 unique values this hour.

A seven-day TimescaleDB retention policy removes old shared estimator windows.

## Projection

Projected unique cardinality scales the observed count through the end of a
one-hour horizon only after:

```text
>= 10 samples
>= 30 seconds elapsed
```

The projection is a trend warning, not a billing forecast.

## Replay isolation

Incident Replay and the Cost Simulator deliberately do **not** use the shared
production estimator.

They use the same hourly-window semantics with a local bounded tracker. This
prevents historical analysis from changing the live production firewall while
keeping policy behavior comparable.

## API

Current active state:

```text
GET /api/v1/cardinality/state?mode=active&limit=30
```

Shadow state:

```text
GET /api/v1/cardinality/state?mode=shadow&limit=30
```

The response includes:

- observed unique count;
- projected unique count;
- unique-growth rate per minute;
- sample count;
- first/last seen; and
- current hourly window.

Internal budget-aggregation pseudo-sources are excluded from this endpoint.

## CLI

```bash
telemetryctl cardinality top \
  --mode active \
  --limit 25 \
  --tenant default
```

## Failure behavior

The distributed estimator uses the same PostgreSQL dependency as durable policy
evidence.

A database failure becomes a transient policy-processing error. The worker uses
its bounded retry/backoff policy; if the dependency does not recover, the normal
terminal DLQ path applies.

TelemetryForge does not silently fall back to replica-local production state,
because doing so would make enforcement semantics change during an outage.

## Operational cost

Shared consistency is not free. v1.2.0 performs one shared-state upsert for each
tracked dimension plus each matching series budget.

This is a deliberate correctness-first implementation. Future scale work can
batch/aggregate register deltas before flushing while preserving merge
semantics.


## Top-dimension query bound

`GET /api/v1/cardinality/state` computes HLL estimates in Go, not with an opaque
SQL extension. To keep the operator query bounded, TelemetryForge reads at most
5,000 recently active current-hour states (scaled down for small requested
limits), computes their projections, then returns the requested top results.

This is an operational ranking surface, not an unbounded analytics scan.
