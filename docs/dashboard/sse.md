# Server-Sent Events

The v0.6.0 dashboard receives live telemetry through:

```text
GET /api/v1/live
```

## Why SSE instead of WebSockets?

The dashboard needs a one-way stream from server to browser. Server-Sent Events
already provide:

- browser reconnect behavior
- a simple text event protocol
- normal HTTP infrastructure compatibility
- less application state than a bidirectional WebSocket connection

WebSockets remain appropriate if TelemetryForge later needs interactive
bidirectional sessions.

## Why poll TimescaleDB behind SSE?

It would be tempting to keep an in-memory list of newly accepted events inside
the gateway. That breaks as soon as there are multiple gateway or worker
replicas because a browser would only see data known to one process.

v0.6.0 instead polls the durable telemetry store once per second per connected
SSE client. That is deliberately simple and horizontally correct.

The trade-off is query cost. Later releases can replace the polling source with
PostgreSQL notifications, Kafka fan-out, Redis, or another shared live-event
bus while keeping the browser-facing SSE contract.

## Event format

```text
event: telemetry
id: <canonical event id>
data: <canonical JSON envelope>
```

A heartbeat comment is emitted every 15 seconds to keep intermediaries from
treating an idle stream as dead.


## Reconnect cursor

Live ordering uses the database `ingested_at` timestamp plus canonical event ID,
not the source-supplied `event_time`.

That distinction matters because telemetry can arrive late or out of order.

The SSE `id` field contains both cursor values:

```text
id: 2026-09-09T16:32:10.123456Z|<event-id>
```

Browsers send the last SSE ID on reconnect using `Last-Event-ID`. The gateway
parses that cursor and resumes after the last delivered stored record.
