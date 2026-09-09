# Local Development

## Requirements

The easiest path requires Docker with Docker Compose.

Direct service development uses Go 1.25+ plus reachable Kafka and
PostgreSQL/TimescaleDB instances.

## Start the full stack

```bash
docker compose up --build
```

Compose starts:

1. TimescaleDB/PostgreSQL.
2. the idempotent `db-migrate` service.
3. Kafka in KRaft mode.
4. Kafka topic initialization (`telemetry.raw`, `telemetry.metrics`,
   `telemetry.dlq`).
5. the Go gateway.
6. the Go processing worker.

The gateway listens on `http://localhost:8080`.

## Why is there a migration service?

PostgreSQL's init directory runs only when the database volume is empty.
`db-migrate` reruns the idempotent SQL migrations on startup so an existing
local volume can move forward between TelemetryForge releases.

## Useful commands

```bash
make kafka-groups
make db-events
make dlq-tail
```

Run services directly:

```bash
go run ./cmd/gateway
go run ./cmd/worker
```

Use operations tooling:

```bash
go run ./cmd/telemetryctl incident freeze --help
go run ./cmd/telemetryctl dlq replay --help
go run ./cmd/telemetryctl dedup prune --help
```

## Verification

```bash
make check
```

With Kafka/TimescaleDB running:

```bash
make integration-test
```

GitHub Actions additionally runs Kafka and storage integration jobs.
