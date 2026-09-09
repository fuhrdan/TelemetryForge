# TelemetryForge Roadmap

Every milestone should remain runnable, documented, testable, and honest about
what is implemented versus planned.

| Version | Status | Milestone |
|---|---|---|
| v0.1.0 | Released | Gateway foundation and canonical envelope |
| v0.2.0 | Released | Kafka durable publishing |
| v0.3.0 | Released | Consumer groups, bounded workers, backpressure |
| v0.4.0 | Released | PostgreSQL/TimescaleDB persistence and query API |
| v0.5.0 | Released | Retry classification, DLQ, replay CLI foundation, Flight Recorder |
| v0.6.0 | Released | Real-time dashboard and automatic incident capture |
| v0.7.0 | Folded into v0.8.0 | Repository hardening, Kubernetes, Cardinality Firewall foundation |
| v0.8.0 | Released | Cardinality Firewall, Terraform, policy-as-code, shadow pipeline |
| **v0.9.0** | **Current release** | **Incident Replay, cost simulation, self-observability, broker lag, k6 methodology** |
| v1.0.0 | Planned | Evidence Graph, security hardening, production-grade portfolio release |

## Why there is no v0.7.0 release tag

The v0.7.0 development branch became a reliability/repository hardening
baseline. Its Kubernetes/Cardinality Firewall work shipped cumulatively in
v0.8.0 instead of creating a misleading intermediate release tag.

## v0.9.0 delivered

- isolated Incident Replay against frozen evidence
- event-time policy replay
- optional dedicated `telemetry.replay*` publication
- Telemetry Cost Simulator
- explicit versioned pricing input
- no dollar estimate without pricing
- OpenTelemetry gateway/worker traces
- Kafka Trace Context propagation
- Prometheus gateway/worker metrics
- broker-derived consumer-group lag
- OpenTelemetry Collector
- Prometheus/Grafana/Tempo local stack
- provisioned TelemetryForge Grafana dashboard
- k6 smoke/sustained/backpressure scenarios
- benchmark methodology without invented results
- replay/cost history in the Next.js dashboard

## v1.0.0 target

### Evidence Graph

- correlate frozen/replayed telemetry by correlation/trace/source/time
- distinguish supporting from contradicting evidence
- show deployment/change relationships where evidence exists
- avoid causal claims unsupported by captured data

### Security

- authentication
- tenant isolation
- authorization
- hardened secrets/TLS profiles
- Kafka TLS/SASL deployment examples
- protected metrics/tracing/admin surfaces
- policy/incident access boundaries
- PII classification/redaction strategy

### Product/release polish

- complete incident investigation workflow
- replay and cost analysis linked directly to incident evidence
- production deployment profile/runbook
- benchmark result artifact produced from a documented environment
- upgrade/rollback runbook
- final GitHub/portfolio demo scenario
