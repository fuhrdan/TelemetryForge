# HTTP API Reference

Base local URL:

```text
http://localhost:8080
```

All JSON responses use `Content-Type: application/json` except the SSE stream.
The current API is a development contract and is not authenticated yet.


## Authentication

In local `disabled` mode the API behaves as a trusted `default` tenant.

In production `api_key` mode use:

```text
Authorization: Bearer <key>
```

or for simple ingestion clients:

```text
X-TelemetryForge-Key: <key>
```

Scopes:

- `ingest` — POST telemetry
- `read` — tenant-scoped API reads
- `admin` — all scopes plus administrative surfaces

`GET /health` and `GET /ready` remain public.

Clients must not send `tenant_id`; the gateway assigns it after authentication.


## Health

### `GET /health`

Process-liveness response:

```json
{"status":"ok"}
```

### `GET /ready`

Checks the gateway's required Kafka dependency. Returns `200` when ready or
`503` when Kafka cannot be reached.

## Ingestion

### `POST /api/v1/events`

Accepts one canonical event envelope. The gateway rejects unknown top-level
fields and request bodies over 1 MiB.

Success:

```text
202 Accepted
```

```json
{"id":"<event-id>","status":"accepted"}
```

The `202` means Kafka acknowledged the record; it does not mean downstream
worker persistence has already completed.

### `POST /api/v1/metrics`

Uses the same envelope but requires `value`.

See [the canonical event format](event-format.md).

## Stored telemetry queries

### `GET /api/v1/events`

### `GET /api/v1/metrics`

Supported filters:

| Parameter | Meaning |
|---|---|
| `source` | Exact source |
| `type` | Exact event type |
| `from` | Inclusive RFC3339 start |
| `to` | Inclusive RFC3339 end |
| `limit` | 1-1000, default 100 |

Results are newest first. The metrics endpoint only returns rows with a numeric
metric value.

## Dashboard summary

### `GET /api/v1/dashboard/summary?window=5m`

The window must be between one minute and 24 hours.

Example response:

```json
{
  "window_seconds": 300,
  "events": 1421,
  "events_per_second": 4.736,
  "error_count": 8,
  "error_rate": 0.0056,
  "p95_latency_ms": 187.4,
  "active_sources": 7
}
```

## Incidents

### `GET /api/v1/incidents?limit=25`

Returns newest detected/frozen incidents. Limit range: 1-100.

### `GET /api/v1/incidents/{id}/events`

Returns the chronological canonical envelopes captured in one frozen incident
window (currently capped by the server at 500 events for this UI route).

## Live telemetry

### `GET /api/v1/live`

Server-Sent Events stream backed by shared durable storage.

Typical stream entry:

```text
event: telemetry
id: 2026-09-09T16:32:10.123456Z|<event-id>
data: {"id":"<event-id>","source":"checkout-api",...}
```

The browser can reconnect using `Last-Event-ID`. See the
[SSE design document](../dashboard/sse.md).

## Common error responses

Validation errors use HTTP `400`:

```json
{"error":"source is required"}
```

Required downstream dependency failures use HTTP `503`:

```json
{"error":"streaming backend unavailable"}
```

The API deliberately avoids returning database/Kafka internals to callers;
detailed dependency errors belong in service logs.


## Cardinality Firewall

### `GET /api/v1/cardinality/findings`

Returns newest-first active and shadow cardinality findings.

Optional:

```text
limit=1..500
```

Each result includes the policy/version, mode, source, event type, dimension,
observed/projected unique estimate, action, reason, short value fingerprint,
and first/last seen timestamps.

Raw high-cardinality values are intentionally not returned.

## Shadow policy

### `GET /api/v1/policy/shadow-diffs`

Returns active-versus-candidate action differences.

Optional:

```text
limit=1..500
```

The shadow pipeline never changes the active event path. This endpoint shows
what the candidate policy would have done differently.


## Replay history

### `GET /api/v1/replays`

Returns newest-first Incident Replay history.

Optional:

```text
limit=1..200
```

The API exposes aggregate replay effects and status; it does not return another
copy of the frozen telemetry payload.

## Cost simulation history

### `GET /api/v1/cost-simulations`

Returns newest-first cost simulation history.

Optional:

```text
limit=1..200
```

Dollar fields are absent when no pricing model was supplied.

## Self-observability

### `GET /metrics`

Gateway Prometheus metrics.

The worker exposes its own `GET /metrics` on the admin listener (default
`:8081`).

Prometheus labels intentionally avoid raw request paths and telemetry IDs.


## Evidence Graph

### `GET /api/v1/incidents/{id}/evidence-graph`

Builds a tenant-scoped Evidence Graph from frozen incident events plus that
tenant's replay/cost history.

The graph returns:

- nodes
- supporting / contradicting / related edges
- evidence basis for each edge
- bounded built-in hypotheses
- an explicit non-causality disclaimer

The latest graph snapshot is persisted for investigation history.


## Schema Intelligence

### `GET /api/v1/schemas`

Returns the newest observed declared version for each tenant-scoped source/type.

Optional:

```text
limit=1..500
```

### `GET /api/v1/schema-history`

Required query parameters:

```text
source
type
```

Returns every observed declared version for that producer identity.

### `GET /api/v1/schema-drift`

Returns newest-first deduplicated drift findings.

### `GET /api/v1/schema-diff`

Required query parameters:

```text
source
type
from
to
```

Returns a field-level compatibility comparison between two persisted declared
versions.

Schema APIs require `read` scope in `api_key` mode and inherit the authenticated
tenant. They cannot query another tenant by supplying a tenant query parameter.


## Distributed cardinality state

### `GET /api/v1/cardinality/state`

Returns current shared one-hour state for the authenticated tenant.

Query parameters:

```text
mode=active|shadow
limit=1..500
```

The response contains observed/projected unique cardinality, growth per minute,
sample count, and timing metadata.

Internal budget aggregation state is not exposed here.

## Cardinality budgets

### `GET /api/v1/cardinality/budgets`

Returns recent active/shadow policy budget consumption for the authenticated
tenant.

Fields include budget scope, policy/version, series limit, observed/projected
unique series, percentage consumed, and status.


## Telemetry Router

### `GET /api/v1/routing/destinations`

Returns tenant-visible destination health plus pending/retry/DLQ counts.

### `GET /api/v1/routing/deliveries`

Optional query parameters:

```text
status=pending|sending|retry|delivered|dead_letter
limit=1..500
```

Returned event envelopes pass through the normal API redactor.

### `GET /api/v1/routing/shadow-diffs`

Returns recent active-versus-candidate destination-set differences. The table
does not duplicate event payloads.

### `GET /api/v1/routing/dead-letters`

Optional:

```text
destination=<name>
limit=1..500
```

Dead-letter envelopes are redacted on API output; stored recovery evidence is
full fidelity.

## Adaptive shaping

### `GET /api/v1/shaping/stats`

Returns tenant-scoped minute aggregates. Optional `window` defaults to `1h` and
is capped at 720 hours.

### `GET /api/v1/shaping/shadow-diffs`

Returns newest active-versus-candidate shaping differences.


## Incident archive import provenance

### `GET /api/v1/archive-imports`

Returns newest-first tenant-scoped metadata about `.tfincident` archives that
were imported into the current tenant.

Optional:

```text
limit=1..200
```

The response contains archive ID, source tenant/incident, imported incident ID,
archive format/product version, whole-file SHA-256, encrypted flag, and import
time.

It does **not** return archive bytes or full-fidelity telemetry.


## Change Intelligence

### `POST /api/v1/changes`

Requires `ingest`. Records a structured deployment/rollback/release/feature-flag/configuration/infrastructure change through the normal Kafka durability boundary. Tenant identity is assigned from authentication.

### `GET /api/v1/changes`

Returns tenant-scoped structured change markers.

### `POST /api/v1/changes/{id}/analyze`

Computes and stores a bounded before/after analysis. Optional `before_seconds` and `after_seconds` are each 60..7200 seconds.

### `GET /api/v1/changes/{id}/analysis`

Returns the latest generated analysis snapshot.

### `GET /api/v1/incidents/{id}/changes`

Returns changes near the frozen incident window.

The change-analysis response explicitly treats timing relationships as observational evidence rather than root-cause proof.

## Connector Platform and OTLP/HTTP JSON

`POST /v1/logs` and `POST /v1/metrics` accept OTLP/HTTP JSON under `ingest`. `GET /api/v1/connectors/catalog` returns built-in capabilities; `GET /api/v1/connectors/runtime` returns recent secret-free router connector heartbeats.


## Operational proof

### `GET /api/v1/proofs`

Returns newest-first verified operational proof records. Each row includes the parsed proof artifact, whole-file SHA-256, artifact byte count, and database recorded time. The endpoint is read-only; proof generation/recording remains an operator CLI workflow.

## Evidence-First Intelligence

### `GET /api/v1/incidents/{id}/investigation`

Rebuilds the incident Evidence Graph from frozen events, stored replay/cost history and nearby structured changes, then returns the deterministic v2 investigation and stores the latest tenant-scoped snapshot.

### `GET /api/v1/incidents/{id}/compare?other=INC-17`

Returns reproducible Evidence Graph similarity plus matching node IDs. Similarity is not a root-cause assertion.

### `GET /api/v1/investigations?limit=25`

Returns newest-first stored investigation snapshots for the authenticated tenant.
