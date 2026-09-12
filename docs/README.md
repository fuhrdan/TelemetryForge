# TelemetryForge Documentation

This directory is the human-readable companion to the source code. The goal is
for a reviewer to understand why the system behaves as it does without having
to reconstruct every decision from implementation details.

## Start here

- [Global Routing Mesh](mesh/global-routing-mesh.md)
- [Replicated Durability](edge/replicated-durability.md)
- [Durable Edge Ingestion](edge/durable-edge.md)

- [Architecture overview](architecture/overview.md)
- [Event flow](architecture/event-flow.md)
- [Configuration reference](reference/configuration.md)
- [Local development](development/local-development.md)
- [Testing strategy](development/testing.md)
- [Troubleshooting](operations/troubleshooting.md)
- [Project roadmap](../ROADMAP.md)
- [v1.1.0 delivered scope](roadmap/v1.1.0.md)
- [v1.2.0 delivered scope](roadmap/v1.2.0.md)
- [v1.3.0 delivered scope](roadmap/v1.3.0.md)

## Architecture

- [Overview](architecture/overview.md)
- [Worker model](architecture/worker-model.md)
- [Backpressure](architecture/backpressure.md)
- [Failure handling](architecture/failure-handling.md)
- [Event flow](architecture/event-flow.md)

## Kafka

- [Topic strategy](kafka/topic-strategy.md)
- [Partitioning](kafka/partitioning.md)
- [Delivery semantics](kafka/delivery-semantics.md)

## Storage

- [Schema](storage/schema.md)
- [Retention](storage/retention.md)
- [Querying](storage/querying.md)

## Reliability and incidents

- [Retry policy](reliability/retries.md)
- [Dead-letter queue](reliability/dlq.md)
- [Deduplication lifecycle](reliability/dedup-lifecycle.md)
- [Incident Flight Recorder](reliability/flight-recorder.md)
- [Automatic incident capture](incidents/automatic-capture.md)


## Evidence-First Intelligence

- [Evidence-First Investigator](intelligence/evidence-first-investigator.md)
- [Incident Comparison](intelligence/incident-comparison.md)
- [Policy Recommendations](intelligence/policy-recommendations.md)
- [v2.0.0 delivered scope](roadmap/v2.0.0.md)
- [v2.1.0 delivered scope](roadmap/v2.1.0.md)
- [v2.2.0 delivered scope](roadmap/v2.2.0.md)
- [v2.3.0 delivered scope](roadmap/v2.3.0.md)
- [Durable Edge ingestion](edge/durable-edge.md)

## Evidence Graph

- [Evidence Graph](evidence/evidence-graph.md)

## Security

- [Authentication](security/authentication.md)
- [Tenant isolation](security/tenant-isolation.md)
- [API redaction](security/redaction.md)
- [Dashboard server proxy](security/dashboard-proxy.md)

## Change Intelligence

- [Change Intelligence](change-intelligence/change-intelligence.md)
- [Change Intelligence API](change-intelligence/change-api.md)
- [Change analysis runbook](operations/change-analysis.md)
- [v1.7.0 delivered scope](roadmap/v1.7.0.md)

## Portable incident evidence

- [Portable incident archives](incidents/portable-archive.md)
- [Archive encryption](security/archive-encryption.md)
- [Archive workflow](operations/archive-workflow.md)

## Replay and cost

- [Incident Replay](incidents/replay.md)
- [Telemetry Cost Simulator](cost/cost-simulator.md)

## Self-observability

- [Self-observability](observability/self-observability.md)
- [Prometheus metrics catalog](observability/metrics.md)
- [OpenTelemetry tracing](observability/tracing.md)
- [Benchmark methodology](performance/benchmark-methodology.md)

## Schema Intelligence

- [Schema Intelligence](schema/schema-intelligence.md)
- [OpenTelemetry semantic conventions](schema/opentelemetry-semantic-conventions.md)

## Cardinality Intelligence

- [Distributed Cardinality Intelligence](cardinality/distributed-cardinality.md)
- [Cardinality Budgets](cardinality/budgets.md)

## Telemetry Router

- [Telemetry Router](routing/telemetry-router.md)
- [Shadow Routing](routing/shadow-routing.md)

## Policy and cardinality

- [Cardinality Firewall](policy/cardinality-firewall.md)
- [Policy-as-Code](policy/policy-as-code.md)
- [Shadow Pipeline](policy/shadow-pipeline.md)

## Deployment

- [Kubernetes](deployment/kubernetes.md)
- [Terraform / AWS EKS](deployment/terraform.md)
- [Production profile](deployment/production-profile.md)

## Dashboard

- [Dashboard overview](dashboard/overview.md)
- [Server-Sent Events](dashboard/sse.md)

## Operations

- [`telemetryctl`](operations/telemetryctl.md)
- [Troubleshooting](operations/troubleshooting.md)

## Development

- [Local development](development/local-development.md)
- [Testing](development/testing.md)
- [Release process](development/release-process.md)
- [GitHub repository conventions](development/github-repository.md)

## Reference

- [HTTP API](api/http-api.md)
- [Event format](api/event-format.md)
- [Configuration](reference/configuration.md)
- [Technology versions](reference/versions.md)

## Architecture Decision Records

ADRs are numbered sequentially in [`adr/`](adr/). Add an ADR when a change
materially affects deployment, reliability, data semantics, security boundaries,
or long-term maintainability.


## v1 operations and demo

- [v1.0 upgrade runbook](operations/upgrade-v1.md)
- [v1.0 rollback runbook](operations/rollback-v1.md)
- [v1.0 portfolio demo](demo/v1-portfolio-demo.md)

## Adaptive sampling and shaping

- [Adaptive Sampling](shaping/adaptive-sampling.md)
- [Shaping Policy](shaping/shaping-policy.md)
- [Visibility Preview](shaping/visibility-preview.md)
- [v1.4.0 delivered scope](roadmap/v1.4.0.md)
- [v1.5.0 delivered scope](roadmap/v1.5.0.md)

## Connector Platform

- [Connector Platform](connectors/connector-platform.md)
- [Connector SDK](connectors/connector-sdk.md)
- [OTLP/HTTP JSON](connectors/otlp.md)
- [Prometheus Remote Write](connectors/prometheus-remote-write.md)
- [Vendor adapters](connectors/vendor-adapters.md)
- [Connector operations](operations/connectors.md)
- [v1.8.0 delivered scope](roadmap/v1.8.0.md)

## Operational proof

- [Operational proof artifacts](proof/operational-proof.md)
- [HA / failure scenarios](proof/ha-scenarios.md)
- [v1.9 benchmark methodology](proof/benchmark-methodology-v1.9.md)
- [v1.9.0 delivered scope](roadmap/v1.9.0.md)

- [High availability](ha/high-availability.md)
- [Failure / chaos testing](operations/chaos-testing.md)
- [Disaster recovery proof](operations/disaster-recovery.md)
- [Kubernetes rolling proof](operations/rolling-proof.md)
