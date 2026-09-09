# Local Development

## Requirements

- Docker with Docker Compose, or
- Go 1.25+ plus a reachable Kafka broker.

## Easiest path

```bash
docker compose up --build
```

Compose starts:

1. Kafka in KRaft mode.
2. `kafka-init`, which creates the telemetry topics.
3. the HTTP gateway.
4. the v0.3.0 processing worker.

The gateway is available on `http://localhost:8080`.

## Run services directly

With Kafka already running:

```bash
go run ./cmd/gateway
```

In another terminal:

```bash
go run ./cmd/worker
```

## Watch the consumer group

```bash
make kafka-groups
```

Kafka reports partitions, current offsets, log-end offsets, and lag. Lag is an
important v0.3.0 operational signal because the worker queue is deliberately
bounded.

## Tune the worker

```bash
TELEMETRYFORGE_WORKER_COUNT=8 \
TELEMETRYFORGE_WORKER_QUEUE_CAPACITY=512 \
go run ./cmd/worker
```

Increase worker count when processing is CPU-bound or safely parallelizable.
Increase queue capacity only to absorb short bursts; Kafka should remain the
durable place for sustained backlog.

## Verification

```bash
make check
```

With Kafka running:

```bash
make integration-test
```
