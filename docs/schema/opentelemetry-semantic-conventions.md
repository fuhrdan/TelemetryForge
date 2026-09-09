# OpenTelemetry Semantic-Convention Awareness

TelemetryForge v1.1.0 adds a deliberately focused compatibility catalog for
high-value OpenTelemetry semantic attributes.

It is **not** a bundled copy of the entire OpenTelemetry semantic-conventions
repository.

The implementation is aligned with the OpenTelemetry Semantic Conventions
1.44.0 documentation available when v1.1.0 was built.

## Why keep a focused catalog?

The upstream semantic-conventions registry is large and evolves independently
of TelemetryForge.

Hard-coding every attribute would create two problems:

- TelemetryForge would quickly become a stale second source of truth; and
- every upstream semantic-convention addition would become an application
  release requirement.

Instead, v1.1 recognizes a focused set useful for schema health and migration
warnings.

Examples include:

```text
service.name
deployment.environment.name
http.request.method
http.response.status_code
network.protocol.version
server.address
server.port
url.scheme
```

## Legacy migration awareness

Common older names are recognized so a producer can be warned rather than
silently treated as an unrelated custom attribute.

Examples:

```text
http.method
  -> http.request.method

http.status_code
  -> http.response.status_code

net.protocol.name
  -> network.protocol.name

deployment.environment
  -> deployment.environment.name
```

The finding is informational/advisory. TelemetryForge does not rewrite the
application's telemetry automatically in v1.1.

## Value types

OpenTelemetry semantic conventions define expected attribute types.

TelemetryForge's canonical `tags` map intentionally stores string labels, so
v1.1 does not flag a tag solely because its semantic convention would normally
have an integer type.

JSON payload paths retain native JSON types. When a recognized semantic
attribute is represented in payload JSON, Schema Intelligence can flag an
unexpected type.

## `schema_url`

The canonical event may now contain:

```json
{
  "schema_version": "2.4",
  "schema_url": "https://opentelemetry.io/schemas/<semconv-version>"
}
```

These have different meanings:

- `schema_version` — application-declared TelemetryForge schema history key;
- `schema_url` — OpenTelemetry semantic-convention schema identifier.

Changing `schema_url` while keeping the same application `schema_version`
creates a warning because the semantic interpretation changed without a
corresponding application schema declaration.

TelemetryForge v1.1 stores `schema_url` as metadata only and **does not dereference
or fetch it**.

## Future direction

A later release can consume upstream OpenTelemetry schema transformations or a
versioned external catalog rather than expanding the built-in compatibility
subset indefinitely.
