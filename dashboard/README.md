# TelemetryForge Dashboard

**Package version:** `1.0.0`
**Release:** `v1.0.0`

The dashboard is intentionally small and readable.

## Stack

- Next.js 16.3.4
- React 19.2.8
- TypeScript 5.9.3
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

The current UI only needs one lightweight live signal chart. Adding a large chart
library at this point would increase dependency/security surface without solving
a real need.

When zooming, time-axis exploration, multiple synchronized series, or large
datasets become requirements, this decision should be revisited.


## v0.8.0 additions

The dashboard now also renders:

- Cardinality Firewall findings
- active/shadow finding mode
- observed/projected unique values
- policy action
- candidate-policy disagreements


## v0.9.0 additions

The dashboard now also renders:

- Incident Replay history
- changed/dropped/quarantined replay counts
- Telemetry Cost Simulator history
- projected monthly volume
- sample-series reduction
- explicit pricing-model output when configured

Deep runtime metrics/traces live in the provisioned Grafana dashboard on
`http://localhost:3001`.


## v1.0.0 additions

- Evidence Graph investigation surface
- supporting/contradicting evidence and hypotheses
- server-side authenticated `/telemetry-api/*` proxy

Production runtime settings:

```text
TELEMETRYFORGE_API_BASE
TELEMETRYFORGE_DASHBOARD_API_KEY
```

The raw dashboard key remains server-side and should have `read` scope only.
