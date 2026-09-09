# TelemetryForge Self-Observability

A telemetry control plane must be observable itself.

v0.9.0 instruments the gateway and worker with Prometheus metrics and
OpenTelemetry traces.

## Local stack

`docker compose up --build` adds:

```text
Prometheus      http://localhost:9090
Grafana         http://localhost:3001
Tempo           http://localhost:3200
OTLP gRPC       localhost:4317
OTLP HTTP       localhost:4318
```

Local Grafana credentials:

```text
admin / telemetryforge
```

These are development credentials only.

## Prometheus

Gateway:

```text
GET http://localhost:8080/metrics
```

Worker:

```text
GET http://localhost:8081/metrics
```

The worker admin port is primarily an internal operational surface.

Prometheus scrapes both services every five seconds in the local stack.

## OpenTelemetry

The Go services export sampled traces over OTLP/HTTP when:

```text
TELEMETRYFORGE_OTLP_TRACES_ENDPOINT
```

is configured.

Docker Compose sends traces to the OpenTelemetry Collector, which batches them
and forwards them to Tempo.

The current trace sampler records 10% of root traces while respecting parent
sampling decisions.

## Trace propagation

The gateway injects W3C Trace Context and Baggage into Kafka record headers.

The consumer extracts those headers and starts the worker-processing span as a
child/continuation of the incoming request trace.

This connects:

```text
HTTP ingestion
    -> Kafka publish
    -> Kafka consume
    -> worker processing
```

without placing trace IDs into Prometheus labels.

## Metrics/cardinality discipline

TelemetryForge applies the same cardinality discipline to its own metrics that
it expects from user telemetry.

HTTP metric labels use the `net/http` **route pattern**, not raw URL paths.

For example:

```text
GET /api/v1/incidents/{id}/events
```

is one metric label value regardless of how many incident IDs exist.

Kafka lag uses only topic and partition labels.

Raw event IDs, correlation IDs, incident IDs, source names, and tag values are
not Prometheus labels in the checked-in metrics.

## Local Tempo

Tempo runs monolithically with local filesystem storage for development.

This is not the recommended production storage architecture. Production Tempo
should use an appropriate object-storage backend and authentication/network
controls.
