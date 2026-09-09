# TelemetryForge Architecture — v0.3.0

TelemetryForge is split into two independently scalable runtime services.

## Ingestion tier

The Go gateway validates the canonical event envelope and publishes accepted
events to Kafka. It stays stateless so additional gateway instances can be
placed behind a load balancer.

## Processing tier

The Go worker joins the `telemetryforge-processors` Kafka consumer group.
Records are decoded and submitted to a bounded worker pool. Workers run
processors and acknowledge records only after successful processing.

```text
HTTP clients
    |
    v
Gateway replicas
    |
    v
Apache Kafka
    |
    v
telemetryforge-processors consumer group
    |
    v
bounded worker queues
    |
    v
processors
```

## Why split the services?

Traffic does not arrive and process at identical rates. Kafka provides a
durable boundary between the two. Gateways can scale for HTTP concurrency while
workers scale for processing throughput.

## Current processing stage

The first processor normalizes source and event-type whitespace. The behavior
is deliberately modest; v0.3.0 proves the concurrency, acknowledgement, and
backpressure architecture before persistence is introduced.

## What comes next?

v0.4.0 adds PostgreSQL/TimescaleDB persistence. Because v0.3.0 already commits
Kafka offsets after successful processing, persistence can be inserted into the
processor path without changing the ingestion API.
