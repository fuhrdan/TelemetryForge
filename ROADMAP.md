# TelemetryForge Roadmap

| Version | Status | Milestone |
|---|---|---|
| v0.1.0 | Released | Gateway foundation and canonical envelope |
| v0.2.0 | Released | Kafka durable publishing |
| v0.3.0 | Released | Consumer groups, bounded workers, backpressure |
| v0.4.0 | Released | PostgreSQL/TimescaleDB persistence |
| v0.5.0 | Released | Retries, DLQ, Flight Recorder |
| v0.6.0 | Released | Dashboard and automatic incident capture |
| v0.7.0 | Folded into v0.8.0 | Reliability/repository hardening |
| v0.8.0 | Released | Kubernetes, Cardinality Firewall, Terraform, policy/shadow pipeline |
| v0.9.0 | Released | Incident Replay, Cost Simulator, self-observability, real Kafka lag, k6 methodology |
| v1.0.0 | Released | Evidence Graph, auth/tenant isolation, redaction, Kafka security, production profile/runbooks |
| v1.1.0 | Delivered development milestone | Schema Intelligence, schema history/drift, OpenTelemetry semantic-convention awareness |
| **v1.2.0** | **Current development release** | **Distributed Cardinality Intelligence, hourly series budgets, forecasting** |
| v1.3.0 | Planned | Multi-destination Telemetry Router |
| v1.4.0 | Planned | Adaptive Sampling & Telemetry Shaping |
| v1.5.0 | Planned | Portable `.tfincident` incident archives |
| v1.6.0 | Planned | Policy lifecycle, approvals, promotion, rollback |
| v1.7.0 | Planned | Change / deployment intelligence |
| v1.8.0 | Planned | Connector and plugin platform |
| v1.9.0 | Planned | Scale, HA, chaos and operational proof |
| v2.0.0 | Planned | Evidence-first intelligent telemetry control plane |

## v1.2.0 delivered

### Shared cardinality state

- production PostgreSQL/TimescaleDB-backed cardinality tracker
- atomic HLL register merge across worker replicas
- exact first-16 hashed uniques
- tenant/active-shadow/source/type/dimension/hour key
- seven-day shared-state retention
- same hourly semantics in isolated replay/cost trackers

### Cardinality forecasting

- observed unique counts
- one-hour projected unique counts
- unique growth/minute
- top exploding dimensions API/dashboard/CLI

### Series budgets

- versioned policy budgets
- tenant-wide and source-pattern scopes
- full series-identity estimation
- warning / critical / exceeded thresholds
- active/shadow policy budget visibility
- non-destructive budget contract

### Operations

- `telemetryctl cardinality top`
- `telemetryctl cardinality budgets`
- distributed state/budget APIs
- dashboard panels
- integration coverage for shared state across independent tracker instances
- ADRs 0028-0029

## v1.3.0 next

Build a multi-destination Telemetry Router with destination-isolated retry,
health, DLQ, fan-out, fallback, and shadow routing policy.
