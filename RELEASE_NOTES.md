# TelemetryForge v0.9.0 Release Notes

## Incident Replay, Cost Simulation & Self-Observability

v0.9.0 turns the Incident Flight Recorder and Cardinality Firewall into a safer
change-analysis workflow while making TelemetryForge observable as a distributed
system itself.

## Incident Replay

Added side-effect-free replay of frozen incident telemetry.

Default replay runs:

```text
frozen incident
    -> normalize
    -> active policy
    -> optional shadow policy
    -> compact replay history
```

It intentionally does not invoke:

- primary telemetry persistence
- Flight Recorder capture
- automatic incident detection
- production Kafka publication

Optional Kafka output requires an explicit topic in the
`telemetry.replay` namespace. The safety check exists in both the library and
`telemetryctl`.

Replay policy evaluation uses the original frozen event timeline rather than
wall-clock replay speed.

## Telemetry Cost Simulator

Added policy-impact analysis against frozen incident samples.

Always available:

- canonical sample bytes
- exact distinct sample series
- active/shadow changed-event counts
- projected 30-day volume

Dollar estimates are omitted unless an operator supplies a strict pricing JSON
file.

The result stores its assumptions so a directional estimate cannot silently
masquerade as a vendor bill.

## Cardinality estimator refinement

The firewall now counts the first 16 distinct SHA-256-derived hashes exactly
before switching to the bounded 64-register HyperLogLog estimator.

This preserves fixed/bounded memory while making small policy thresholds
deterministic for replay and simulation.

## Prometheus self-observability

Added gateway and worker metrics including:

- HTTP request rate/latency by registered route pattern
- accepted telemetry
- Kafka publish failures
- worker outcomes
- worker processing duration
- retries
- queue depth/capacity
- broker-derived Kafka consumer lag
- DLQ counts

Metric labels intentionally exclude raw URL paths, IDs, arbitrary sources, and
telemetry tags.

## OpenTelemetry tracing

Added sampled OpenTelemetry traces for:

- gateway HTTP requests
- Kafka publish
- worker processing
- individual worker pipeline stages

W3C Trace Context/Baggage is injected into Kafka headers and extracted by the
consumer.

## Broker-derived consumer lag

The worker now queries Kafka group lag independently every 15 seconds.

The lag gauge represents broker end offset versus committed consumer-group
offset, not a local fetch-size approximation.

## Local observability stack

Docker Compose now provisions pinned:

- OpenTelemetry Collector 0.160.0
- Prometheus 3.13.3
- Grafana 13.2.1
- Tempo 3.0.2

Grafana is provisioned with Prometheus/Tempo datasources and a
TelemetryForge-specific self-observability dashboard.

## k6 methodology

Added pinned k6 2.2.0 scenarios:

- smoke
- sustained ingestion
- backpressure

The repository documents what must be recorded before publishing a performance
result.

v0.9.0 does **not** invent or advertise a universal requests-per-second number.

## Dashboard

Added:

- Incident Replay history
- replay changed/dropped/quarantined counts
- cost simulation history
- monthly volume projection
- sample-series reduction
- explicit price-model output when present

## Database

Migration `007_replay_cost.sql` adds durable replay and cost-simulation history.

## API

Added:

```text
GET /api/v1/replays
GET /api/v1/cost-simulations
GET /metrics
```

## CI

Added validation for:

- observability configuration
- Prometheus config
- OpenTelemetry Collector config
- Tempo config
- Grafana dashboard JSON
- k6 scenario inspection

Existing Go/Kafka/TimescaleDB/dashboard/Kubernetes/Terraform/container checks
remain.

## Known limitations

- Cardinality estimator state remains replica-local; cluster-global
  cardinality may be underestimated across many workers.
- Incident Replay currently exercises normalization and policy behavior rather
  than every future processing/output connector.
- Cost simulation uses canonical JSON sample size and explicit assumptions; it
  does not model backend compression, wire framing, aggregation, retention
  tiers, or a vendor contract unless supplied.
- `telemetry.replay` has no checked-in consumer; publication is an explicit
  isolation/export mechanism.
- Prometheus/Tempo/Grafana/OTLP endpoints in Compose are local development
  surfaces with development credentials/no production authentication.
- Tempo uses local filesystem storage in the local stack; object storage is
  recommended for production.
- No benchmark results are published yet because a representative
  dependency-backed environment result has not been captured.
- Authentication, tenant isolation, hardened Kafka TLS/SASL, and PII redaction
  remain v1.0 work.
- A registry-generated dashboard `package-lock.json` is still required for
  fully reproducible npm transitive dependencies.
