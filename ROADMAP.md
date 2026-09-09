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
| **v1.0.0** | **Current release** | **Evidence Graph, auth/tenant isolation, redaction, Kafka security, production profile/runbooks** |

## v1.0.0 delivered

### Evidence Graph
- evidence nodes from frozen telemetry/replay/cost analysis
- shared correlation/trace evidence
- temporal/source context
- latency-before-error evidence
- deployment/change-before-error association
- explicit recovery contradiction
- supporting / contradicting / related classifications
- hypothesis status without automated root-cause claims
- API, CLI, dashboard, persisted snapshot

### Security
- scoped API-key authentication
- hash-only API-key server configuration
- authentication-derived tenant identity
- client tenant spoofing rejection
- tenant-scoped database state
- tenant-scoped Cardinality Firewall/incident state
- server-side dashboard read-key proxy
- presentation-time redaction
- Kafka TLS/mTLS/SASL PLAIN/SCRAM

### Deployment / operations
- Kubernetes production overlay
- non-root/seccomp/service-account-token hardening
- authenticated metrics guidance
- upgrade runbook
- rollback runbook
- final portfolio demo

## After v1.0

Future work should be evidence-driven rather than roadmap inflation.

Potential directions include:

- shared/distributed Cardinality Firewall estimator state
- richer schema registry and lineage
- policy signing/rotation
- tenant-specific retention/redaction policy
- external observability output connectors
- benchmark result artifacts from representative environments
- optional evidence-first AI explanations that cite Evidence Graph nodes and
  preserve contradictions
