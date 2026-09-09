# TelemetryForge Architecture

TelemetryForge is a distributed telemetry control plane built around explicit
durability, bounded concurrency, evidence preservation, and reversible policy
changes.

## Runtime components

```text
                  HTTP
Applications ----------------> Go Gateway
                                  |
                                  v
                              Apache Kafka
                                  |
                                  v
                         Consumer Group / Workers
                                  |
                                  v
                         Flight Recorder (full)
                                  |
                                  v
                              Normalize
                                  |
                                  v
                      Cardinality Firewall
                         /             \
                        /               \
                 active policy       shadow policy
                    |                    |
          mutate normal event      compare only
                    |                    |
                    +---------+----------+
                              |
                              v
                    PostgreSQL/TimescaleDB
                              |
               +--------------+--------------+
               |                             |
               v                             v
          Query / SSE API             Incident evidence
               |
               v
         Next.js Dashboard
```

Terminal processing failures go to `telemetry.dlq`.

## Core guarantees

1. **Durable acceptance.** The gateway returns `202` only after Kafka
   acknowledges the event.
2. **Bounded memory.** Worker queues, incident detector state, and cardinality
   estimator state have explicit caps.
3. **At-least-once processing.** Kafka offsets advance only after successful
   handling.
4. **Contiguous partition commits.** Concurrent worker completion cannot commit
   past earlier unfinished records in the same partition.
5. **Rebalance-safe poll batches.** A bounded poll batch retains partition
   ownership until all submitted jobs complete.
6. **Idempotent persistence.** Canonical event IDs prevent duplicate primary
   telemetry rows after replay/retry.
7. **Durable terminal failure handling.** Source offsets advance only after the
   DLQ replacement is acknowledged.
8. **Evidence before mutation.** The Flight Recorder captures the event before
   normalization or cardinality policy removes dimensions.
9. **Shadow before promotion.** Candidate policy can evaluate real traffic
   without changing the active event representation.

## Ingestion

The Go gateway:

- validates a versioned canonical envelope;
- applies request-size and strict-JSON limits;
- publishes to Kafka;
- exposes health/readiness;
- provides bounded read APIs;
- provides dashboard summaries/incidents;
- streams live telemetry over SSE; and
- exposes Cardinality Firewall / shadow-policy evidence.

## Kafka

Topics:

```text
telemetry.raw
telemetry.metrics
telemetry.dlq
```

Records are keyed by source.

Consumer auto-commit is disabled. The acknowledgement coordinator advances a
partition only through its completed contiguous prefix.

franz-go `BlockRebalanceOnPoll` keeps ownership stable while one bounded
`PollRecords` batch is still being processed.

See `docs/kafka/delivery-semantics.md`.

## Worker pipeline

Current stage order:

```text
Flight Recorder
    -> Normalizer
    -> Cardinality Firewall / active + shadow policy
    -> Idempotent Persister
    -> Automatic Incident Detector
```

### Why policy runs before persistence

The normal stored representation is the representation that future output
routers should be allowed to forward.

`drop_tag` therefore removes a dangerous dimension before normal persistence.
The original value remains preserved in the earlier Flight Recorder.

`quarantine` additionally stores a full normalized copy in dedicated quarantine
storage and marks the normal representation as blocked for future routing.

## Cardinality state

Each source/type/dimension uses a 64-register HyperLogLog estimator.

State is:

- fixed-size per estimator;
- globally bounded by configured dimension count;
- least-recently-seen evicted at the cap;
- value-hashed with SHA-256; and
- operational findings rate-limited.

The current estimator is replica-local. Cluster-global merging is a documented
future scaling improvement.

## Policy

Policy files are JSON and versioned in Git.

```text
policies/active.json
policies/shadow.json
```

Active actions:

```text
allow
drop_tag
quarantine
```

The shadow policy observes the same normalized event but never changes the
active path.

## Storage

PostgreSQL/TimescaleDB holds:

- canonical telemetry
- event-ID deduplication
- Flight Recorder rolling buffer
- frozen incidents
- quarantine evidence
- cardinality findings
- active/shadow policy differences

## Dashboard

The browser only communicates with the Go API. It does not connect directly to
Kafka or PostgreSQL.

SSE reads shared durable storage using `(ingested_at, event_id)` as a cursor,
which keeps the browser contract correct across gateway replicas.

## Kubernetes

`deployments/kubernetes/base/` contains a Kustomize base with:

- gateway/worker/dashboard Deployments
- Services
- health/readiness probes
- HPA examples
- PodDisruptionBudgets
- policy ConfigMap
- secret template

Worker replica usefulness is bounded by Kafka partition count.

## Terraform

`infra/terraform/aws/` provisions AWS networking plus EKS.

Kafka and TimescaleDB remain external service contracts so the compute
foundation does not dictate organization-specific data-platform choices.

## Deployment versions

- upstream Kubernetes baseline: 1.37
- Amazon EKS Terraform default: 1.36
- Terraform: 1.16.2
- AWS provider: 6.62.0

## Next architecture milestone

v0.9.0 adds:

- Incident Replay
- Telemetry Cost Simulator
- OpenTelemetry self-observability
- Prometheus/Grafana metrics
- broker-derived Kafka lag
- reproducible k6 performance methodology
