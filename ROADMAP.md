# TelemetryForge Roadmap

The roadmap is intentionally incremental: every release should be runnable,
documented, testable, and understandable on its own.

| Version | Status | Milestone |
|---|---|---|
| v0.1.0 | Released | Gateway foundation and canonical event envelope |
| v0.2.0 | Released | Kafka durable publishing |
| v0.3.0 | Released | Consumer groups, bounded workers, backpressure |
| v0.4.0 | Released | PostgreSQL/TimescaleDB persistence and query API |
| v0.5.0 | Released | Retry classification, DLQ, replay CLI, Flight Recorder |
| v0.6.0 | Released | Real-time dashboard and automatic incident capture |
| **v0.7.0** | **In development** | Kubernetes scaling and Cardinality Firewall |
| v0.8.0 | Planned | Terraform, policy-as-code, shadow pipeline |
| v0.9.0 | Planned | Incident Replay, cost simulation, self-observability, load testing |
| v1.0.0 | Planned | Evidence Graph, security hardening, polished production portfolio release |

## v0.7.0 target

v0.7.0 has two themes:

### Kubernetes and horizontal scaling

- Kubernetes manifests for gateway, worker, and dashboard
- ConfigMaps and Secrets boundaries
- health/readiness probes
- resource requests/limits
- HorizontalPodAutoscaler examples
- PodDisruptionBudgets
- worker scaling guidance tied to Kafka partition count
- failure/scaling documentation and smoke checks

### Cardinality Firewall

- detect label/tag cardinality growth before forwarding it downstream
- identify likely dangerous dimensions such as session IDs or request IDs
- estimate projected unique-series growth
- allow, drop-tag, or quarantine policy actions
- surface findings in the dashboard
- preserve the rejected/quarantined evidence needed for debugging
- document false-positive and policy trade-offs

The detailed acceptance checklist lives in
[`docs/roadmap/v0.7.0.md`](docs/roadmap/v0.7.0.md).

## v1.0 direction

TelemetryForge is not trying to win by reproducing every dashboard feature of
Datadog, Grafana, Splunk, or Honeycomb. Its differentiators are the control
plane between applications and those backends:

- Incident Flight Recorder
- safe Incident Replay
- Cardinality Firewall
- Telemetry Cost Simulator
- Evidence Graph with supporting and contradicting evidence
- policy-as-code and shadow evaluation before deployment
