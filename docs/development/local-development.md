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
8. active/shadow policy files packaged with the worker image.

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


## Policy validation

```bash
make policy-check
```

## Cardinality demo

```bash
make demo-cardinality
```

The demo emits unique `request_id` and `session_id` tags. Open the dashboard and
watch the Cardinality Firewall and Shadow Pipeline panels.

## Kubernetes

```bash
make k8s-render
```

## Terraform

```bash
make terraform-check
```


## v0.9.0 observability stack

The full Compose stack also starts:

```text
Prometheus        http://localhost:9090
Grafana           http://localhost:3001
Tempo             http://localhost:3200
OTLP gRPC         localhost:4317
OTLP HTTP         localhost:4318
```

Local Grafana credentials:

```text
admin / telemetryforge
```

The gateway and worker export sampled traces to the local OpenTelemetry
Collector through:

```text
TELEMETRYFORGE_OTLP_TRACES_ENDPOINT=http://otel-collector:4318/v1/traces
```

Direct `go run` development leaves the variable empty by default, so tracing
does not require a Collector unless explicitly enabled.

## Replay / cost workflow

1. Generate traffic.
2. Freeze or automatically capture an incident.
3. Run:

```bash
go run ./cmd/telemetryctl incident replay \
  --id <INCIDENT_ID>
```

4. Compare telemetry shape:

```bash
go run ./cmd/telemetryctl cost simulate \
  --incident <INCIDENT_ID>
```

5. Refresh the dashboard to see replay/cost history.

## Load scenarios

```bash
make load-smoke
make load-sustained
make load-backpressure
```

The Make targets use the pinned `grafana/k6:2.2.0` image.

For sustained/backpressure tests:

```bash
RATE=250 DURATION=5m make load-sustained
```

Watch Grafana while the test runs. The backpressure scenario is expected to make
Kafka lag visible when input exceeds a modest local worker/database capacity.

Do not turn that one local result into a universal throughput claim.


## Schema Intelligence demo

```bash
make demo-schema
```

Then open the TelemetryForge dashboard and find **Schema Intelligence**.

The demo establishes 24 normal `orders-api / order.created` v1 observations,
then sends same-version type/missing-field drift and finally a declared v2
breaking shape.

Direct CLI inspection:

```bash
go run ./cmd/telemetryctl schema inspect   --source orders-api   --type order.created
```
