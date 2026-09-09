# TelemetryForge v1.2.0 Release Notes

## Distributed Cardinality Intelligence

v1.2.0 turns the Cardinality Firewall from a replica-local warning mechanism
into a shared cluster control.

Production workers now merge cardinality observations through fixed-size hourly
state in PostgreSQL/TimescaleDB. A dimension therefore sees the same shared
estimate regardless of which Kafka partition/worker replica observed the value.

## Shared estimator

The cluster key is:

```text
tenant_id
active/shadow mode
source
event_type
dimension
hourly window
```

Each row stores:

- 64 HLL registers;
- up to 16 SHA-256-derived hashes for exact low-cardinality counting;
- first/last observation timestamps; and
- sample count.

Raw tag values are never persisted in shared cardinality state.

Each observation atomically raises one HLL register with
`max(existing,incoming)`, which is the merge operation required to combine
replica observations safely.

## Hourly policy semantics

Dimension thresholds now apply to the current one-hour UTC window.

Projection still requires at least ten samples and thirty seconds before
scaling observed unique values to the one-hour horizon.

Incident Replay and the Cost Simulator use local bounded trackers with the same
hourly window semantics. Historical analysis never writes production
cardinality state.

## Series budgets

Policy JSON can now define versioned hourly series budgets.

Example:

```json
{
  "name": "tenant-hourly-series",
  "source": "*",
  "type": "*",
  "series_limit": 50000,
  "warning_percent": 70,
  "critical_percent": 90
}
```

Budget identity hashes the full telemetry series:

```text
source + event type + sorted non-TelemetryForge tags
```

This measures distinct series shapes rather than incorrectly summing individual
label cardinalities.

Statuses:

```text
healthy
warning
critical
exceeded
```

Budgets are advisory. They do not drop, quarantine, or sample telemetry.
Explicit dimension policy remains the mutation mechanism.

## Dashboard

Added **Distributed Cardinality** and **Series Budgets** panels showing:

- top projected-growth dimensions;
- observed/projected uniques;
- unique growth per minute;
- active hourly window;
- budget scope;
- active/shadow policy version;
- budget consumption percentage; and
- healthy/warning/critical/exceeded state.

## API

Added:

```text
GET /api/v1/cardinality/state?mode=active&limit=30
GET /api/v1/cardinality/budgets?limit=100
```

Both remain tenant-scoped through the v1 authentication boundary.

## CLI

Added:

```bash
telemetryctl cardinality top --mode active --limit 25 --tenant default
telemetryctl cardinality budgets --tenant default
```

`telemetryctl policy validate` now reports configured budget count and validates
budget names/limits/threshold percentages.

## Database

Migration `009_distributed_cardinality.sql` adds:

- `cardinality_cluster_state` TimescaleDB hypertable;
- seven-day shared-state retention; and
- `cardinality_budget_status` current-status table.

## Policy defaults

The checked-in active/shadow policies now include:

- a tenant-wide hourly series budget; and
- a narrower checkout-source hourly series budget.

The shadow policy intentionally uses lower candidate budgets so operators can
compare pressure before promotion.

## Failure behavior

TelemetryForge does not silently fall back from distributed production state to
replica-local state during a database outage.

A shared-state write failure is treated as a transient policy dependency
failure and goes through normal bounded retry/DLQ handling if it does not
recover.

## Known limitations

- Shared cardinality consistency adds database write pressure: one atomic state
  update per tracked dimension and matching budget.
- HLL remains approximate after the first 16 unique hashes.
- Forecasting is intentionally a simple one-hour linear trend heuristic.
- Budget status is advisory and not a vendor billing model.
- Cross-region database latency/partition behavior is not yet the v1.9 HA
  architecture.
