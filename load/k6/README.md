# k6 Load Scenarios

TelemetryForge v0.9.0 adds reproducible **methodology**, not invented benchmark
numbers.

The scenarios use k6 2.2.0 and submit the same canonical metric envelope as a
normal client.

## Requirements

Start TelemetryForge:

```bash
docker compose up --build
```

The Makefile runs k6 in Docker. The test container reaches the host-published
gateway on `host.docker.internal:8080`.

## Smoke

```bash
make load-smoke
```

Purpose:

- verify the load harness
- validate HTTP `202` acceptance
- detect obvious latency/error regressions

Default:

```text
1 VU
30 seconds
```

## Sustained ingestion

```bash
make load-sustained
```

Default:

```text
100 requests/second
2 minutes
```

Override without changing the checked-in scenario:

```bash
RATE=250 DURATION=5m make load-sustained
```

## Backpressure

```bash
make load-backpressure
```

Default:

```text
500 requests/second
90 seconds
```

This scenario is intentionally capable of creating consumer lag on modest
development machines. Watch:

```text
http://localhost:3001
```

for:

- worker queue depth
- Kafka consumer lag
- worker processing time
- retries / DLQ
- gateway request latency

A healthy bounded design can accumulate Kafka lag under excess input. That is
different from allowing an unbounded in-process queue to consume memory.

## Results

Do not copy one laptop's numbers into the README as universal throughput claims.

When publishing a benchmark, record at minimum:

- commit/tag
- CPU and RAM
- Docker/host platform
- Kafka partition count
- worker replicas and worker count
- database resources
- test script
- RATE / DURATION
- p50/p95/p99 request latency
- HTTP error rate
- maximum consumer lag
- whether the backlog drained after load stopped

See `docs/performance/benchmark-methodology.md`.
