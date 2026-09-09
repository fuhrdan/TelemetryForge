# TelemetryForge v0.6.0 Release Notes

> These notes describe the immutable `v0.6.0` tag. Development changes for v0.7.0 are tracked under **Unreleased** in `CHANGELOG.md` and in `docs/roadmap/v0.7.0.md`.

## Real-Time Dashboard & Automatic Incident Capture

v0.6.0 is the first visual TelemetryForge release.

### Added

- Next.js 16.3.4 / React 19.2.7 TypeScript dashboard
- responsive operations UI
- Server-Sent Events telemetry feed
- durable-store-backed live polling
- five-minute dashboard summary API
- events/sec card
- error-rate card
- P95 latency card
- active-source card
- live SVG metric chart
- live telemetry table
- frozen incident list
- incident event timeline
- automatic high-latency incident capture
- automatic error-burst incident capture
- per-source/reason cooldown
- incident trigger metadata
- configurable latency/error thresholds
- dashboard Docker image and Compose service
- incident detail API
- dependency-free demo telemetry/incident generator
- human-readable SSE/dashboard/incident documentation
- ADR 0011: SSE from durable shared storage
- ADR 0012: explicit automatic incident thresholds

### Automatic capture defaults

High latency:

```text
>= 1000ms for latency/duration metrics
```

Error burst:

```text
5 error-classified events from one source in 1 minute
```

The freeze window looks back 15 minutes and the default duplicate-trigger
cooldown is 10 minutes.

### Live-stream architecture

The SSE endpoint reads from shared durable storage rather than a gateway-local
broadcast queue. This is intentionally less clever but remains correct when
multiple application replicas exist.

### Dashboard dependency policy

The first chart is implemented with React + SVG rather than bringing in a
charting library. This keeps the v0.6.0 frontend small and makes the live data
path easy to inspect.

### Known limitations

- SSE currently polls TimescaleDB once per second per connected browser.
- thresholds are global defaults, not source-specific rules.
- automatic capture freezes the preceding window; it does not yet continue
  capturing a post-trigger tail.
- incident status cannot yet be changed from the UI.
- incident timeline does not yet correlate causal dependencies.
- dashboard authentication/tenant isolation is not implemented.
- there is no Cardinality Firewall yet.

These limitations are documented rather than hidden.
