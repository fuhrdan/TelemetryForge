# Dashboard Overview

The dashboard is TelemetryForge's visual operations surface.

The dashboard is a Next.js/TypeScript application served separately from the Go
gateway. It talks to the gateway through a same-origin Next.js rewrite so the
browser does not need direct cross-origin access to the API.

## What the dashboard shows

The main screen contains:

- events per second over the last five minutes
- error rate
- P95 latency for millisecond latency/duration metrics
- active telemetry source count
- live metric values
- live SSE event stream
- frozen incident list
- captured incident timeline

## Why no charting library yet?

The first chart is a small SVG sparkline written in React. This keeps the
dependency surface small and makes the data path easy to review.

A larger visualization library can be justified later when the dashboard needs
zooming, brushing, stacked series, or higher-density exploration.

## API routes used

```text
GET /api/v1/dashboard/summary
GET /api/v1/live
GET /api/v1/incidents
GET /api/v1/incidents/{id}/events
```

The Next.js application proxies them under `/telemetry-api/` so the browser
talks to one origin.


## Cardinality Firewall

The v0.8 dashboard adds a panel showing recent active/shadow findings:

- source
- dimension
- observed unique estimate
- projected unique estimate
- policy action

## Shadow Pipeline

A second panel shows candidate-policy disagreements:

```text
active action -> shadow action
```

This gives operators a visual review surface before promoting a shadow policy.
