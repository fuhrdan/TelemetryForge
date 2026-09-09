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
| **v1.1.0** | **Current release** | **Schema Intelligence, schema history/drift, OpenTelemetry semantic-convention awareness** |
| v1.2.0 | Planned | Distributed Cardinality Intelligence |
| v1.3.0 | Planned | Multi-destination Telemetry Router |
| v1.4.0 | Planned | Adaptive Sampling & Telemetry Shaping |
| v1.5.0 | Planned | Portable `.tfincident` incident archives |
| v1.6.0 | Planned | Policy lifecycle, approvals, promotion, rollback |
| v1.7.0 | Planned | Change / deployment intelligence |
| v1.8.0 | Planned | Connector and plugin platform |
| v1.9.0 | Planned | Scale, HA, chaos and operational proof |
| v2.0.0 | Planned | Evidence-first intelligent telemetry control plane |

## v1.1.0 delivered

### Schema registry

- tenant-scoped source/type/version registry
- optional OpenTelemetry `schema_url`
- bounded payload field discovery
- deterministic value-free schema fingerprints
- event-ID idempotent observations
- schema history by application-declared version

### Drift intelligence

- additive field detection
- same-version JSON type conflicts
- required-field inference after an observation floor
- breaking removal of established required fields
- cross-version compatibility diff
- sticky per-version health (`healthy`, `warning`, `breaking`)

### OpenTelemetry awareness

- focused high-value semantic-attribute catalog
- stable attribute recognition
- legacy HTTP/deployment/network migration warnings
- payload JSON semantic type checks
- separate application `schema_version` from OpenTelemetry `schema_url`

### Operations

- fail-open schema registration by default
- `telemetryctl schema inspect`
- `telemetryctl schema diff`
- schema registry/history/drift APIs
- Schema Health dashboard
- deterministic `make demo-schema`

## v1.2.0 next

Move Cardinality Firewall estimation from replica-local state toward a shared,
tenant-aware cluster view with budgets, trend forecasting, and consistent
decisions across workers.

See the 1.x -> 2.0 roadmap in the repository history for the longer product
sequence.
