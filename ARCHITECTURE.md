# TelemetryForge Architecture

TelemetryForge is a distributed telemetry control plane built around explicit
durability, bounded concurrency, evidence preservation, reversible policy
changes, and replayable incident evidence.

## Runtime components

```text
Applications
    |
    v
Go Gateway
    |
    v
Apache Kafka
    |
    v
Consumer Group / Workers
    |
    v
Flight Recorder
    |
    v
Normalize
    |
    v
Cardinality Firewall
   / \
  /   \ candidate policy
 v     v
active shadow
  \   /
   v v
TimescaleDB
   |
   +--------------------+
   |                    |
   v                    v
Incidents          Query / SSE API
   |                    |
   +-----> Replay       v
   |       / Cost    Next.js Dashboard
   |
   +-----> Evidence

Gateway / Worker
   |
   +---- Prometheus metrics
   |
   `---- OpenTelemetry traces
             |
             v
      OTel Collector -> Tempo

Prometheus ------------------> Grafana
Tempo -----------------------> Grafana
```

Terminal processing failures go to `telemetry.dlq`.

Optional replay publication is isolated under `telemetry.replay*`.

## Core guarantees

1. **Durable acceptance.** HTTP `202` is returned only after Kafka acknowledges
   the accepted event.
2. **Bounded application memory.** Worker queues, incident detector state,
   cardinality estimators, and finding-suppression maps have explicit limits.
3. **At-least-once processing.** Kafka offsets advance after successful normal
   or durable DLQ handling.
4. **Contiguous partition commits.** Concurrent worker completion cannot commit
   past earlier unfinished records from the same partition.
5. **Rebalance-safe poll batches.** Partition ownership remains stable until all
   submitted work from one bounded poll batch finishes.
6. **Idempotent primary persistence.** Canonical event IDs absorb duplicate
   at-least-once deliveries.
7. **Evidence before mutation.** The Flight Recorder captures telemetry before
   normalization or policy removes dimensions.
8. **Shadow before promotion.** Candidate policy never mutates the active event
   path.
9. **Replay isolation.** Frozen incident analysis does not re-enter the normal
   production persistence/incident path.
10. **No implicit pricing.** Cost simulation produces money only from explicit
    pricing input.
11. **Self-observability uses bounded labels.** Raw telemetry IDs/tags/URLs do
    not become Prometheus dimensions.

## Ingestion

The Go gateway owns the public HTTP boundary. It:

- validates the canonical event envelope;
- applies request-size/strict-JSON limits;
- produces to Kafka and waits for acknowledgement;
- exposes bounded read APIs;
- serves dashboard/incident/replay/cost history;
- streams live telemetry over SSE;
- exposes `/health`, `/ready`, and `/metrics`; and
- emits sampled OpenTelemetry spans.

HTTP metrics use registered `net/http` route patterns rather than raw paths.

## Kafka boundary

Topics:

```text
telemetry.raw
telemetry.metrics
telemetry.dlq
telemetry.replay
```

The first three are normal processing topics. `telemetry.replay` is an isolated,
opt-in replay-output namespace with no checked-in normal worker consumer.

Production ingestion records are keyed by source.

The consumer disables auto-commit and coordinates completed offsets per
partition. A later offset cannot move the committed position while an earlier
record is unfinished.

A bounded `PollRecords` batch also retains partition ownership until its
submitted jobs finish.

## Trace propagation

The gateway starts an HTTP span. The producer starts a Kafka publish child span
and injects W3C Trace Context/Baggage into Kafka headers.

The worker extracts that context before starting:

```text
worker.process
```

Each processor stage creates a child:

```text
worker.stage
```

This makes Flight Recorder, policy, persistence, and incident-processing delays
visible in Tempo without turning those stage names into unbounded metrics.

## Worker pipeline

Production stage order:

```text
Flight Recorder
    -> Normalizer
    -> Cardinality Firewall (active + shadow)
    -> Idempotent Persister
    -> Automatic Incident Detector
```

### Cardinality Firewall

Each source/type/dimension uses:

- exact counting for the first 16 distinct SHA-256-derived hashes; then
- a bounded 64-register HyperLogLog estimate.

The global number of tracked dimension states is bounded.

Active actions:

```text
allow
drop_tag
quarantine
```

The shadow policy observes but never mutates production behavior.

## Incident Flight Recorder

The rolling Flight Recorder stores the pre-policy decoded envelope.

A frozen incident copies a selected time window into durable incident storage.

Duplicate processing attempts are deduplicated by canonical event ID when the
incident timeline is frozen/read.

## Incident Replay

Replay is deliberately not the production worker chain.

Default replay executes:

```text
frozen incident
    -> normalize
    -> active policy
    -> optional shadow policy
    -> compact replay-result persistence
```

It does not call:

- the Flight Recorder;
- primary telemetry persistence;
- automatic incident capture; or
- Kafka publish.

If an operator explicitly supplies an output topic, the library only accepts:

```text
telemetry.replay
telemetry.replay.<suffix>
```

Replay policy evaluation uses the frozen event timestamp so time-window behavior
represents the incident rather than replay execution speed.

## Telemetry Cost Simulator

The simulator uses the same frozen evidence.

It computes:

- baseline/active/shadow canonical JSON bytes;
- exact distinct sample series;
- changed-event counts;
- linear 30-day byte-volume projection.

It does not assign prices unless a strict operator-provided pricing model exists.

Pricing assumptions are persisted with every simulation.

## Storage

PostgreSQL/TimescaleDB holds:

- canonical telemetry
- event-ID deduplication
- rolling Flight Recorder
- frozen incidents
- quarantine evidence
- cardinality findings
- active/shadow policy differences
- replay run history
- compact replay event results
- cost simulation results/assumptions

## Self-observability

Each Go service uses a private Prometheus registry plus Go/process collectors.

Important gateway metrics include:

```text
HTTP request count / duration
accepted events
Kafka publish failures
```

Worker metrics include:

```text
job outcomes / duration
retries
queue depth / capacity
Kafka consumer lag
DLQ counts
```

Kafka lag is broker-derived using committed consumer-group offsets versus broker
end offsets.

The lag monitor runs independently every 15 seconds. Failure to collect lag is
operationally visible but does not stop processing.

## Local observability stack

Docker Compose provisions:

```text
OpenTelemetry Collector
Prometheus
Grafana
Tempo
```

The Collector receives OTLP and forwards traces to Tempo.

Prometheus scrapes the gateway, worker, and Collector.

Grafana receives provisioned Prometheus and Tempo datasources plus a
TelemetryForge dashboard.

Tempo uses local filesystem storage only for development.

## Dashboard

The Next.js UI reads only Go APIs; it never connects directly to Kafka or
PostgreSQL.

It displays:

- live telemetry/summary
- frozen incidents
- Cardinality Firewall findings
- shadow-policy differences
- replay history
- cost simulation history

Deep runtime metrics/traces live in Grafana/Tempo rather than duplicating a
second observability query engine inside the Next.js app.

## Kubernetes

The Kustomize base contains:

- gateway/worker/dashboard Deployments
- Services
- health/readiness probes
- `/metrics` scrape annotations
- HPA examples
- PodDisruptionBudgets
- policy ConfigMap
- secret template

The worker admin Service exposes port 8081 inside the cluster for operational
scraping/readiness.

Worker replica usefulness remains bounded by Kafka partition count.

## Terraform

The AWS Terraform foundation provisions networking and EKS compute.

Kafka, TimescaleDB, Prometheus, Tempo, and other data services remain external
contracts so organizations can use their approved managed/self-hosted
platforms.

## Performance methodology

k6 scenarios are checked in for:

- smoke
- sustained ingress
- bounded-backpressure behavior

The repository does not advertise a throughput number until the result includes
hardware/resources, Kafka/worker settings, request latency/error rate, maximum
broker lag, and backlog-drain evidence.

## v1.0 architecture target

v1.0 should add the Evidence Graph and hardened security boundaries without
weakening the existing replay/policy/durability guarantees.
