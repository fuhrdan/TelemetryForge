# HTTP API Reference

Base local URL:

```text
http://localhost:8080
```

All JSON responses use `Content-Type: application/json` except the SSE stream.
The current API is a development contract and is not authenticated yet.

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
