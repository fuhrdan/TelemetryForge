# Querying Stored Telemetry

TelemetryForge exposes bounded read endpoints backed by TimescaleDB.

## Events

```text
GET /api/v1/events
```

Optional parameters:

- `source`
- `type`
- `from` — RFC3339 timestamp
- `to` — RFC3339 timestamp
- `limit` — 1 through 1000, default 100

Example:

```bash
curl "http://localhost:8080/api/v1/events?source=payment-service&limit=50"
```

## Metrics

```text
GET /api/v1/metrics
```

It accepts the same filters and returns only records containing a metric value.

## Query ordering

Results are newest first.

## Why cap requests at 1000 rows?

Unbounded API queries are an easy way to turn an analytics endpoint into a
database/resource exhaustion problem. TelemetryForge intentionally establishes bounded
queries so the analytics surface cannot request an unlimited result set.

Pagination and aggregate/downsample queries will be added as the analytics
surface matures.
