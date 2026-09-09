# TelemetryForge Roadmap

Every milestone should remain runnable, documented, testable, and honest about
what is implemented versus planned.

| Version | Status | Milestone |
|---|---|---|
| v0.1.0 | Released | Gateway foundation and canonical envelope |
| v0.2.0 | Released | Kafka durable publishing |
| v0.3.0 | Released | Consumer groups, bounded workers, backpressure |
| v0.4.0 | Released | PostgreSQL/TimescaleDB persistence and query API |
| v0.5.0 | Released | Retry classification, DLQ, replay CLI, Flight Recorder |
| v0.6.0 | Released | Real-time dashboard and automatic incident capture |
| v0.7.0 | Folded into v0.8.0 | Repository hardening, Kubernetes, Cardinality Firewall foundation |
| **v0.8.0** | **Released** | **Cardinality Firewall, Terraform, policy-as-code, shadow pipeline** |
| v0.9.0 | Planned | Incident Replay, cost simulation, self-observability, k6 load testing |
| v1.0.0 | Planned | Evidence Graph, security hardening, polished production portfolio release |

## Why there is no v0.7.0 release tag

The v0.7.0 development branch became a repository/reliability hardening baseline.
When work continued directly into v0.8.0, the planned v0.7 Kubernetes and
Cardinality Firewall work was completed cumulatively inside v0.8.0 instead of
creating a release tag that would imply those features had shipped earlier.

## v0.9.0 target

- Incident Replay against frozen production telemetry
- policy replay in shadow mode
- Telemetry Cost Simulator
- OpenTelemetry self-observability
- Prometheus/Grafana operational metrics
- broker-derived consumer lag
- reproducible k6 load tests
- checked-in benchmark methodology/results
- richer incident investigation workflow

## v1.0 direction

The production-grade portfolio release should demonstrate:

- Incident Flight Recorder
- Incident Replay
- Cardinality Firewall
- policy-as-code + shadow evaluation
- Telemetry Cost Simulator
- Evidence Graph
- explicit security boundaries
- reproducible performance evidence
- deployment/runbook quality
