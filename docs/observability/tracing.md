# OpenTelemetry Tracing

TelemetryForge v0.9.0 uses the OpenTelemetry Go SDK and OTLP/HTTP exporter.

## Configuration

Set:

```text
TELEMETRYFORGE_OTLP_TRACES_ENDPOINT=http://collector:4318/v1/traces
```

An empty value disables the application trace exporter.

## Services

Traces identify:

```text
telemetryforge-gateway
telemetryforge-worker
```

with service version `0.9.0`.

## HTTP spans

The gateway middleware creates a span around HTTP requests and records:

- HTTP method
- registered route pattern
- response status
- error status for 5xx responses

The exporter request size is capped.

## Kafka propagation

The publisher injects the configured global Trace Context/Baggage propagator
into Kafka record headers.

The consumer reconstructs the context before handing the event to a worker.

## Worker spans

Each job creates:

```text
worker.process
```

with bounded semantic attributes:

- telemetry source as a trace attribute
- telemetry event type as a trace attribute
- processing outcome
- retry attempt count

Trace attributes are not Prometheus dimensions. They remain subject to tracing
retention/access controls because telemetry source/type may still be sensitive.

## Sampling

The current development default is:

```text
ParentBased(TraceIDRatioBased(0.10))
```

That samples approximately 10% of new root traces while preserving an upstream
sampling decision.

A future deployment profile can expose the ratio as policy/configuration if
production requirements justify it.
