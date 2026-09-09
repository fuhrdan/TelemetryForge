# TelemetryForge Dashboard

The v0.6.0 dashboard is intentionally small and readable.

## Stack

- Next.js 16.3.4
- React 19.2.7
- TypeScript
- native browser `EventSource`
- handwritten SVG chart
- plain CSS

There is no charting or state-management dependency in this release.

## Run locally

Start the Go backend stack, then:

```bash
npm install
npm run dev
```

Open `http://localhost:3000`.

The Next.js rewrite proxies `/telemetry-api/*` to `http://localhost:8080` during
local development.

## Production container

The dashboard Dockerfile builds Next.js standalone output. Docker Compose sets
the internal API destination to `http://gateway:8080`.

## Data model

The UI intentionally mirrors backend API types in a few small TypeScript types
inside `app/page.tsx`. Once the API surface grows, generated/shared contracts
will be preferable.

## Why native SVG?

v0.6.0 only needs one lightweight live signal chart. Adding a large chart
library at this point would increase dependency/security surface without solving
a real need.

When zooming, time-axis exploration, multiple synchronized series, or large
datasets become requirements, this decision should be revisited.
