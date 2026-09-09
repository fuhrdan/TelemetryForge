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
incident-detector state cap are still code-level defaults. Cardinality Firewall
policy is versioned JSON; incident thresholds can move into versioned policy in
a later release.

## Dashboard

The Next.js server proxies `/telemetry-api/*` to the Go gateway at runtime.
`TELEMETRYFORGE_API_BASE` selects that upstream and
`TELEMETRYFORGE_DASHBOARD_API_KEY` supplies the optional read-only server-side
credential. The raw key is never embedded in browser JavaScript.

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


## Authentication / tenant isolation

| Variable | Default | Purpose |
|---|---|---|
| `TELEMETRYFORGE_AUTH_MODE` | `disabled` | `disabled` or `api_key` |
| `TELEMETRYFORGE_API_KEYS_FILE` | `security/api-keys.example.json` | Hash-only API-key document |
| `TELEMETRYFORGE_DEFAULT_TENANT` | `default` | Trusted tenant used only in disabled local mode |
| `TELEMETRYFORGE_TENANT_ID` | `default` | Trusted tenant default for telemetryctl commands |

In `api_key` mode tenant identity comes from the matching API-key record.

Clients must omit `tenant_id` from ingestion JSON.

## API response redaction

| Variable | Default | Purpose |
|---|---|---|
| `TELEMETRYFORGE_REDACT_TAG_KEYS` | `authorization,cookie,email,user_email,token,password,api_key` | Comma-separated tag keys redacted on API/SSE output |
| `TELEMETRYFORGE_REDACT_PAYLOAD` | `false` | Replace API/SSE payload bodies with a redaction marker |

These settings do not destructively modify stored Flight Recorder evidence.

## Kafka transport security

| Variable | Default | Purpose |
|---|---|---|
| `TELEMETRYFORGE_KAFKA_TLS` | `false` | Enable TLS |
| `TELEMETRYFORGE_KAFKA_CA_FILE` | empty | Optional custom CA PEM |
| `TELEMETRYFORGE_KAFKA_CERT_FILE` | empty | Optional mTLS client certificate |
| `TELEMETRYFORGE_KAFKA_KEY_FILE` | empty | Optional mTLS client private key |
| `TELEMETRYFORGE_KAFKA_SASL_MECHANISM` | `disabled` | `plain`, `scram-sha-256`, `scram-sha-512`, or disabled |
| `TELEMETRYFORGE_KAFKA_SASL_USERNAME` | empty | SASL username |
| `TELEMETRYFORGE_KAFKA_SASL_PASSWORD` | empty | SASL password |

Client certificate/key must be configured together.

## Dashboard server proxy

| Variable | Default | Purpose |
|---|---|---|
| `TELEMETRYFORGE_API_BASE` | `http://localhost:8080` in direct development | Upstream Go gateway URL used by Next.js server |
| `TELEMETRYFORGE_DASHBOARD_API_KEY` | empty | Read-only raw gateway key held only by the Next.js server |

Production should use a tenant-scoped `read` key for the dashboard.


## Schema Intelligence

| Variable | Default | Purpose |
|---|---|---|
| `TELEMETRYFORGE_SCHEMA_FAIL_OPEN` | `true` | Continue normal telemetry processing if schema registry persistence fails |

Schema drift itself never rejects telemetry in v1.1.

The canonical event fields used by the registry are:

```text
schema_version   required application-declared schema version
schema_url       optional OpenTelemetry semantic-convention schema URL
```

The required-field inference floor (20 observations / 95% presence), maximum
512 fields per event, and five-level payload walk are v1.1 code-level safety
defaults.


### Schema Intelligence fixed safety bounds

The current release intentionally keeps these as code-level invariants rather
than configuration knobs:

```text
5 payload nesting levels per event
512 discovered fields per event
2,048 accumulated unique field paths per schema version
```

The limits prevent hostile or accidental dynamic field-name generation from
turning schema observation into unbounded application/database state.
