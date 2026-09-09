# TelemetryForge v1.3.0 Development Release Notes

## Telemetry Router

v1.3.0 separates **routing intent** from **destination delivery**. The primary
Kafka worker records a durable destination plan after primary persistence; a
separate router service performs external I/O.

This means one broken observability backend cannot hold a healthy destination or
the core Kafka processing partition hostage.

## Routing policy

Routing JSON can match:

```text
authenticated tenant
source glob
event-type glob
severity
tag key/value globs
```

Matching rules fan out to the union of their destinations. `stop: true` allows
deliberate first-match termination.

If no rule matches, `fallback_destination` can receive the event.

## Durable outbox

Migration `010_telemetry_router.sql` adds:

- `routing_deliveries`
- `routing_shadow_diffs`
- `routing_dead_letters`
- `routing_destination_health`

Outbox uniqueness is:

```text
tenant_id + event_id + destination
```

so bounded worker retry cannot duplicate one destination intent.

## Router service

New binary:

```text
telemetryforge-router
```

Admin surface:

```text
:8082/health
:8082/ready
:8082/metrics
```

Each destination gets its own concurrency lanes. Multiple router replicas claim
work with PostgreSQL `FOR UPDATE SKIP LOCKED` and expiring leases.

Router readiness intentionally does **not** fail because one external destination
is unavailable; that would defeat destination isolation. Backend health is
reported separately.

## Destination types

### Kafka

Uses the same Kafka TLS/SASL environment contract as the rest of TelemetryForge.
Destination brokers can optionally override the cluster default.

### HTTP/webhook

Posts the canonical post-policy event and includes event/tenant/idempotency
headers.

Credential-shaped static headers (`Authorization`, cookies, API keys, tokens,
and secrets) are rejected in routing JSON. Use `bearer_token_env` for Bearer
authentication or `header_env` for generic secret-backed HTTP headers.

## Retry, DLQ, and failure fallback

Each destination stores:

- max attempts
- base/max backoff
- current attempts/status
- next attempt time
- last error

After terminal failure TelemetryForge records a **per-destination routing DLQ**.
An optional `failure_fallback` can atomically enqueue the same event to another
destination such as `archive`. Fallback cycles are rejected.

This routing DLQ is separate from `telemetry.dlq`, which represents failures in
the primary processing pipeline.

## Shadow routing

The candidate routing file is evaluated against the same post-policy event but
never creates destination I/O. Only added/removed destination names are stored.

## API

Added tenant-scoped:

```text
GET /api/v1/routing/destinations
GET /api/v1/routing/deliveries
GET /api/v1/routing/shadow-diffs
GET /api/v1/routing/dead-letters
```

## CLI

Added:

```bash
telemetryctl routing validate --file routing/active.json
telemetryctl routing destinations --tenant default
telemetryctl routing deliveries --status retry --tenant default
telemetryctl routing dlq list --destination security --tenant default
telemetryctl routing dlq requeue --event EVT --destination security --tenant default
```

## Dashboard

Added:

- destination health
- pending/retry/DLQ counts
- candidate shadow-routing differences
- recent per-destination DLQ summary

## Local demo destinations

Compose creates:

```text
telemetry.routed.primary
telemetry.routed.security
telemetry.routed.archive
```

The checked-in active route sends unmatched telemetry to `primary`, production
errors to `primary + security`, and uses `archive` as the failure fallback.

## Known limitations

- External destination delivery is at-least-once, not exactly-once.
- HTTP destinations should implement idempotency using the canonical event ID.
- v1.3 establishes routing semantics but intentionally does not freeze a public
  third-party connector SDK; the connector/plugin platform remains v1.8 work.
- Destination configuration is deployment-global while delivery/health history
  is tenant-scoped.
- The routing outbox uses PostgreSQL; cross-region HA semantics remain v1.9 work.
- Delivered outbox rows and shadow-routing differences do not yet have automatic
  pruning; production operators should define retention before sustained high-volume use.
- A destination should be drained before it is removed/disabled; removing it also
  removes the active dispatch lane for outstanding rows.
