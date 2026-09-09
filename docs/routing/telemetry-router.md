# Telemetry Router

TelemetryForge v1.3.0 adds durable multi-destination routing after the normal
processing/persistence pipeline.

The design goal is simple:

> A broken destination must not stall healthy destinations or hold the core
> Kafka processing partition open.

## Data flow

```text
Kafka worker
  -> Flight Recorder
  -> normalize / schema / cardinality policy
  -> primary TimescaleDB persistence
  -> durable routing outbox
  -> Kafka source acknowledgement

routing outbox
  -> telemetryforge-router
       -> primary Kafka destination
       -> security Kafka destination
       -> HTTP/webhook destination
       -> archive/failure fallback
```

Destination network I/O never runs inside the primary Kafka worker.

## Configuration

Active routing is defined in:

```text
routing/active.json
```

Candidate routing is defined in:

```text
routing/shadow.json
```

Both documents are strict JSON and validated at startup/CI.

A rule can match:

- authenticated tenant
- source glob
- event-type glob
- severity (`severity` or `level` tag; error event types also infer `error`)
- arbitrary tag key/value glob

Matching rules fan out to the union of their destination lists.

`stop: true` can deliberately stop evaluation after one matching rule.

## Destinations

v1.3.0 ships two destination types.

### Kafka

```json
{
  "name": "primary",
  "type": "kafka",
  "enabled": true,
  "topic": "telemetry.routed.primary"
}
```

If `brokers` is omitted, the router uses `TELEMETRYFORGE_KAFKA_BROKERS` and the
same TLS/SASL security profile as gateway/worker/telemetryctl.

### HTTP

```json
{
  "name": "incident-webhook",
  "type": "http",
  "enabled": true,
  "url": "https://example.invalid/telemetry",
  "health_url": "https://example.invalid/health",
  "header_env": {
    "X-API-Key": "TELEMETRYFORGE_WEBHOOK_API_KEY"
  },
  "bearer_token_env": "TELEMETRYFORGE_WEBHOOK_TOKEN"
}
```

HTTP delivery includes:

```text
Content-Type: application/json
Idempotency-Key: <event_id>
X-TelemetryForge-Event-ID: <event_id>
X-TelemetryForge-Tenant-ID: <tenant_id>
```

Secrets should not be stored in routing JSON. Static credential-shaped headers
such as `Authorization`, `Cookie`, API keys, tokens, and secrets are rejected by
validation. Use `header_env` for generic secret-backed headers or
`bearer_token_env` for Bearer authentication. The router verifies configured
secret environments at startup and re-reads them for each request. Standard
container environment variables are normally fixed for the life of the process,
so rotate environment-backed secrets with the deployment platform's normal
restart/rollout mechanism.

## Fallback behavior

`fallback_destination` means "route here when no rule matched."

A destination can also declare:

```json
"failure_fallback": "archive"
```

After the original destination exhausts its bounded attempts, TelemetryForge
atomically:

1. records a per-destination dead letter;
2. marks that delivery terminal; and
3. enqueues the same event for the configured fallback destination.

Fallback cycles are rejected at configuration validation.

## Retry isolation

Retry policy is captured on each durable delivery intent:

```text
max_attempts
base_delay_ms
max_delay_ms
```

Each destination has its own configurable concurrency lane.

For example, if `security` is unavailable while `primary` is healthy:

```text
primary  -> delivered
security -> retry -> retry -> destination DLQ
archive  -> optional fallback
```

`primary` does not wait for `security`.

## Leasing and multiple router replicas

Router replicas claim due deliveries with PostgreSQL:

```text
FOR UPDATE SKIP LOCKED
```

Claims receive a bounded lease. If a router process dies after claiming a row,
another replica can reclaim it after the lease expires.

Delivery remains at-least-once. Destinations should respect the event ID /
Idempotency-Key when they support idempotency.

## Read APIs

```text
GET /api/v1/routing/destinations
GET /api/v1/routing/deliveries
GET /api/v1/routing/shadow-diffs
GET /api/v1/routing/dead-letters
```

All are tenant-scoped by the existing v1 authentication boundary.

## CLI

```bash
telemetryctl routing validate --file routing/active.json
telemetryctl routing destinations --tenant default
telemetryctl routing deliveries --status retry --tenant default
telemetryctl routing dlq list --destination security --tenant default
telemetryctl routing dlq requeue --event EVT-1 --destination security --tenant default
```


## Local demo

```bash
make demo-routing
```

The demo emits production-tagged errors and audit events so the checked-in
active policy visibly produces fan-out and archive deliveries.


## Destination lifecycle

Before removing or disabling a destination, drain or explicitly resolve its
`pending` / `retry` deliveries. Router lanes are created from the active
destination catalog; intentionally removing a destination also removes its
dispatch lane. The tenant-scoped routing APIs make orphaned/non-drained rows
visible for operational cleanup.

Delivered outbox rows and shadow-routing history do not yet have an automatic
pruning policy in v1.3.0-dev. Dead letters are intentionally preserved until an
operator requeues or handles them. Production deployments should establish a
reviewed retention procedure before high-volume use.
