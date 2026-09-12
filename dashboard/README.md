# TelemetryForge Dashboard

**Package version:** `2.3.0`
**Release:** `v2.3.0`

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

The Next.js server-side proxy forwards `/telemetry-api/*` to the configured
TelemetryForge gateway. Direct development defaults to `http://localhost:8080`;
Docker Compose uses `http://gateway:8080`.

## Production container

The dashboard Dockerfile builds Next.js standalone output. The server-side proxy
keeps API credentials out of browser JavaScript and preserves authenticated SSE.

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


## v1.1.0 additions

The dashboard now includes **Schema Intelligence**:

- current healthy/warning/breaking counts;
- latest source/type/version entries;
- field and inferred-required counts;
- clickable declared-version history; and
- recent schema drift findings.


## v1.2.0 additions

The dashboard now includes **Distributed Cardinality Intelligence**:

- top projected-growth dimensions from shared cluster state;
- observed and projected hourly unique values;
- unique-growth-per-minute trends;
- active/shadow policy mode visibility;
- tenant/source series-budget consumption; and
- healthy/warning/critical/exceeded budget state.

Production worker replicas merge the same hourly HLL state; the dashboard is not
summing replica-local estimates.


## v1.3.0 additions

- Telemetry Router destination health and isolated queue panel
- shadow-routing destination differences
- recent per-destination DLQ count

The browser still reaches these APIs through the server-side authenticated proxy.

## v1.4.0 additions

- Adaptive Sampling last-hour event/byte retention
- protected-event and transformation counts
- candidate Shadow Shaping differences
- same server-side authenticated API proxy from v1.0

Sampling policy lives in `shaping/active.json`; `shaping/shadow.json` is
non-destructive candidate evaluation.


## v1.5.0 additions

- Portable Incident Archive import provenance panel
- `.tfincident` operator workflow visibility

Archive export itself remains a `telemetryctl` administrative workflow because
portable incident packages contain full-fidelity evidence and should not be
turned into an ordinary browser download endpoint.


## v1.7.0 additions

- Change Intelligence panel for deployments, releases, rollbacks, feature flags, configuration, and infrastructure changes
- before/after error-rate and p95 analysis
- observed blast-radius estimate
- rollback/recovery evidence
- Git SHA, build, environment, and version context

Change analysis is observational evidence; the dashboard does not label temporal proximity as root cause.

## v1.8.0 additions

- Connector Platform runtime panel
- connector kind/protocol/signal capabilities
- backend readiness independent from router readiness
- router-instance heartbeat visibility

The dashboard receives no connector credentials.


## v1.9.0 additions

- Operational Proof panel for reviewed `.tfproof.json` results
- assertion/measurement summaries
- whole-file artifact SHA-256 provenance

The dashboard displays recorded evidence; it does not invent or extrapolate benchmark claims.

## v2.0.0 additions

- Evidence-First Investigator panel
- finding confidence plus supporting/contradicting citation counts
- replay/cost evidence counts
- advisory policy recommendation surface with explicit human-approval warning

The browser receives cited investigation results through the existing server-side gateway proxy. It does not receive a global lifecycle `control` credential and cannot activate a recommendation.
