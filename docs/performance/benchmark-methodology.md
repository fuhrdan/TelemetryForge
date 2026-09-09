# Benchmark Methodology

TelemetryForge intentionally does not publish a universal requests-per-second
claim in v0.9.0.

Instead, the repository contains repeatable k6 scenarios and a checklist for
publishing evidence that another engineer can interpret.

## k6 baseline

Pinned local/CI version:

```text
k6 2.2.0
```

Scenarios:

```text
load/k6/ingest-smoke.js
load/k6/ingest-sustained.js
load/k6/backpressure.js
```

## Record the environment

Every published result should include:

- TelemetryForge commit/tag
- test script
- k6 version
- host OS
- CPU model / allocated CPU count
- RAM / Docker memory limit
- Docker Desktop / Engine version when relevant
- Kafka version
- Kafka partition count
- gateway replica count
- worker replica count
- worker goroutine count
- worker queue capacity
- TimescaleDB version/resources
- active policy version
- shadow policy enabled/disabled
- test RATE and DURATION

## Record the outcomes

At minimum:

- request count
- HTTP error rate
- p50/p95/p99 HTTP duration
- maximum broker-derived consumer lag
- maximum worker queue depth
- worker processing p95
- retries
- DLQ count
- whether consumer lag returned to zero after load stopped

## Interpreting backpressure

If ingress temporarily exceeds downstream capacity, a bounded design should
prefer:

```text
Kafka lag rises
```

over:

```text
Go heap grows without a configured bound
```

Lag is not automatically a failure. The important questions are:

- Is it visible?
- Does it stabilize under sustainable load?
- Does it drain after the burst?
- Is processing latency still within the intended operational envelope?

## Avoid benchmark theater

Do not:

- run for five seconds and call the peak a sustained throughput number;
- compare results from different hardware without saying so;
- disable durability and compare against durable settings;
- quote one p95 without the error rate;
- hide consumer backlog;
- convert one local result into a production capacity guarantee.

The first checked-in benchmark result should be added only after the actual
environment can run the dependency-backed stack and k6 scenario end to end.
