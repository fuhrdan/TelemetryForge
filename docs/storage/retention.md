# Retention

Telemetry volume grows continuously. Retention is therefore part of the data
model, not a cleanup task to think about later.

## Current development default

The local TimescaleDB migration installs a **30-day retention policy** for
`telemetry_events`.

```sql
SELECT add_retention_policy(
    'telemetry_events',
    INTERVAL '30 days',
    if_not_exists => TRUE
);
```

The value is intentionally visible in SQL so operators can review the behavior
without searching application code.

## Why 30 days?

It is a development default, not a universal production recommendation. It is
long enough to make the local system useful while demonstrating that telemetry
has an explicit lifecycle.

A production deployment should base retention on:

- incident investigation requirements
- compliance requirements
- telemetry value
- storage cost
- source/type
- whether lower-resolution aggregates can replace raw events

Later TelemetryForge releases will connect retention and sampling choices to
the Telemetry Cost Simulator.


## Schema observation ledger

v1.1.0 adds `schema_observations`, a tenant-scoped per-event idempotency ledger.
It should not grow forever.

Recommended maintenance:

```bash
telemetryctl schema prune --older-than 840h
```

The 35-day default intentionally exceeds the 30-day development telemetry
retention window. Pruning observations preserves the accumulated schema registry
and drift history.


## Distributed cardinality state

v1.2.0 stores hourly mergeable estimator rows in the
`cardinality_cluster_state` hypertable.

Retention:

```text
7 days
```

The current policy decision uses only the current hour. Seven days are retained
for operational inspection/debugging without allowing cardinality-control state
to grow indefinitely.

`cardinality_budget_status` stores only the latest status for each
versioned-policy budget and does not require time-series retention.
