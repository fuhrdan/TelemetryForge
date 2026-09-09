# Configuration Reference

TelemetryForge services are configured through environment variables. Docker
Compose supplies local-development defaults; production deployments should use
an external secret/configuration mechanism.

## Gateway

| Variable | Default | Purpose |
|---|---|---|
| `TELEMETRYFORGE_ADDRESS` | `:8080` | HTTP listen address |
| `TELEMETRYFORGE_KAFKA_BROKERS` | `localhost:9092` | Comma-separated Kafka bootstrap brokers |
| `TELEMETRYFORGE_KAFKA_CLIENT_ID` | `telemetryforge-gateway` | Kafka producer client ID |
| `TELEMETRYFORGE_KAFKA_RAW_TOPIC` | `telemetry.raw` | Generic event topic |
| `TELEMETRYFORGE_KAFKA_METRIC_TOPIC` | `telemetry.metrics` | Metric event topic |
| `TELEMETRYFORGE_DATABASE_URL` | local PostgreSQL URL | Query/dashboard database connection |

## Worker

| Variable | Default | Purpose |
|---|---|---|
| `TELEMETRYFORGE_KAFKA_BROKERS` | `localhost:9092` | Kafka bootstrap brokers |
| `TELEMETRYFORGE_WORKER_CLIENT_ID` | `telemetryforge-worker` | Kafka consumer client ID |
| `TELEMETRYFORGE_WORKER_GROUP_ID` | `telemetryforge-processors` | Consumer group |
| `TELEMETRYFORGE_WORKER_TOPICS` | `telemetry.raw,telemetry.metrics` | Topics consumed by workers |
| `TELEMETRYFORGE_WORKER_COUNT` | `4` | Concurrent in-process processors |
| `TELEMETRYFORGE_WORKER_QUEUE_CAPACITY` | `256` | Bounded in-process job queue |
| `TELEMETRYFORGE_WORKER_DLQ_TOPIC` | `telemetry.dlq` | Terminal-failure topic |
| `TELEMETRYFORGE_DATABASE_URL` | local PostgreSQL URL | Persistence connection |

## Automatic incident capture

| Variable | Default | Purpose |
|---|---|---|
| `TELEMETRYFORGE_INCIDENT_LATENCY_MS` | `1000` | Latency/duration metric threshold in milliseconds |
| `TELEMETRYFORGE_INCIDENT_ERROR_COUNT` | `5` | Error-count threshold within the built-in one-minute window |

The current error window, Flight Recorder lookback, cooldown, and 10,000-source
incident-detector state cap are still code-level defaults. The v0.8.0 policy-as-code
format currently governs Cardinality Firewall decisions; incident thresholds can be
migrated into versioned policy in a later release.

## Dashboard

`TELEMETRYFORGE_API_BASE` is a **build-time** Next.js value used by the
same-origin rewrite. Local development defaults to `http://localhost:8080`;
the dashboard container is built with `http://gateway:8080`.

## Security note

Values in `.env.example` and `docker-compose.yml` are intentionally obvious
local defaults. They are not production credentials. See
[the security policy](../../SECURITY.md).


## Cardinality Firewall / policy

| Variable | Default | Purpose |
|---|---|---|
| `TELEMETRYFORGE_POLICY_FILE` | `policies/active.json` for direct development | Active JSON policy |
| `TELEMETRYFORGE_SHADOW_POLICY_FILE` | `policies/shadow.json` | Candidate shadow policy; set to `disabled` to turn off |
| `TELEMETRYFORGE_CARDINALITY_MAX_DIMENSIONS` | `20000` | Maximum in-memory source/type/dimension estimators |
| `TELEMETRYFORGE_WORKER_ADMIN_ADDRESS` | `:8081` | Worker health/readiness listener |

Docker/Kubernetes override the policy paths to
`/etc/telemetryforge/policies/*.json`.

## Kubernetes

The Kustomize base supplies non-secret values through
`telemetryforge-config` and policies through `telemetryforge-policies`.

`TELEMETRYFORGE_DATABASE_URL` comes from the separately managed
`telemetryforge-secrets` Secret.

## Terraform

Terraform configuration is independent of application environment variables.
See `infra/terraform/aws/variables.tf` for AWS-region, EKS version, and node
scaling inputs.


## Self-observability

| Variable | Default | Purpose |
|---|---|---|
| `TELEMETRYFORGE_OTLP_TRACES_ENDPOINT` | empty | Full OTLP/HTTP traces URL; empty disables application trace export |

Docker Compose sets:

```text
http://otel-collector:4318/v1/traces
```

for the gateway and worker.

Prometheus endpoints do not require an environment variable:

```text
Gateway: GET :8080/metrics
Worker:  GET :8081/metrics
```

The checked-in local stack exposes:

```text
Prometheus  :9090
Grafana     :3001
Tempo       :3200
OTLP gRPC   :4317
OTLP HTTP   :4318
```

These endpoints are development defaults, not a production authentication
model.

## Incident Replay / Cost Simulator

The CLI reuses:

```text
TELEMETRYFORGE_DATABASE_URL
TELEMETRYFORGE_KAFKA_BROKERS
```

Replay publication is optional and must target:

```text
telemetry.replay
telemetry.replay.<suffix>
```

The cost simulator accepts pricing only from an explicit `--pricing` JSON file.
There is no environment-default dollar model.
