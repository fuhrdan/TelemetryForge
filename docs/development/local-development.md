# Local Development

## Requirements

The easiest path requires Docker with Docker Compose.

Direct service development uses:

- Go 1.27.1+
- Node.js 24.21 LTS+
- Kafka
- PostgreSQL/TimescaleDB

## Start the complete local stack

```bash
docker compose up --build
```

Compose starts:

1. TimescaleDB/PostgreSQL.
2. idempotent SQL migrations.
3. Kafka in KRaft mode.
4. Kafka topic initialization.
5. the Go gateway.
6. the Go worker.
7. the Next.js dashboard.

Open:

```text
http://localhost:3000
```

Gateway:

```text
http://localhost:8080
```

## Run backend services directly

```bash
go run ./cmd/gateway
go run ./cmd/worker
```

## Run the dashboard directly

From the repository root:

```bash
make dashboard-dev
```

The local Next.js configuration proxies `/telemetry-api/*` to
`http://localhost:8080`.

## Useful operations

```bash
make kafka-groups
make db-events
make dlq-tail
```

## Automatic incident thresholds

```bash
TELEMETRYFORGE_INCIDENT_LATENCY_MS=750 \
TELEMETRYFORGE_INCIDENT_ERROR_COUNT=3 \
go run ./cmd/worker
```

## Verification

Backend:

```bash
make check
```

Dashboard:

```bash
make dashboard-build
```

GitHub Actions runs Go tests/builds, Kafka integration, TimescaleDB integration,
dashboard type checking/build, and container builds.
