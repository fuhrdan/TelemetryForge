# TelemetryForge Architecture Overview

TelemetryForge currently has three application surfaces:

1. Go ingestion/query gateway.
2. Go Kafka processing workers.
3. Next.js operations dashboard.

Shared durable infrastructure is Apache Kafka plus PostgreSQL/TimescaleDB.

## Current data path

```text
client
  |
  v
gateway
  |
  v
Kafka
  |
  v
bounded worker
  |
  v
Flight Recorder
  |
  v
normalize
  |
  v
Cardinality Firewall
  |                \
  |                 `--> shadow-policy comparison
  v
TimescaleDB
  |
  +--> dashboard/query/SSE
  |
  `--> automatic incident detection
```

## Active versus shadow policy

The active policy can change the normal event representation.

The shadow policy receives the same normalized event and calculates what it
would do, but never mutates the event. Differences are persisted for review.

This is the safety mechanism that lets policy changes be evaluated before they
become destructive.

## Scale boundaries

Gateway/dashboard replicas scale independently.

Worker replicas are constrained by Kafka partition ownership. Internal worker
goroutines are a separate concurrency layer.

Cardinality estimates are currently replica-local and bounded. The design
intentionally documents that limitation rather than pretending replica-local
HyperLogLog state is a cluster-global count.

## Deployment

The same runtime contract is available through:

- local Docker Compose;
- Kubernetes Kustomize base; and
- AWS EKS foundation created by Terraform.

Kafka and TimescaleDB endpoints remain configuration contracts across all three
deployment modes.


## Schema Intelligence boundary

Schema Intelligence is an advisory stage between normalization and policy. It
sees the producer shape before `drop_tag`/quarantine decisions, while the Flight
Recorder remains the full-fidelity evidence source before both.

Registry persistence is fail-open by default.
