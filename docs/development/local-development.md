# Local Development

## Prerequisites

- Go 1.25+
- Docker with Docker Compose v2

## Full stack

```bash
docker compose up --build
```

Compose starts:

1. Apache Kafka in single-node KRaft mode.
2. A one-shot `kafka-init` service that creates the v0.2.0 topics.
3. The TelemetryForge gateway after topic initialization succeeds.

Verify:

```bash
curl http://localhost:8080/health
curl http://localhost:8080/ready
```

List topics:

```bash
make kafka-topics
```

## Run the gateway directly

Start Kafka first:

```bash
docker compose up -d kafka kafka-init
```

Then:

```bash
go run ./cmd/gateway
```

The default direct-run broker is `localhost:9092`.

## Unit tests

```bash
make test
```

The HTTP tests use an in-memory Publisher implementation, so Kafka is not required.

## Kafka integration test

With the Compose broker running:

```bash
make integration-test
```

The integration test verifies broker readiness and performs a real produce operation to `telemetry.raw`.
