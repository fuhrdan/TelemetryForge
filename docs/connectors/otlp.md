# OTLP/HTTP JSON

v1.8 adds outbound and inbound OTLP/HTTP JSON support. The `otlp_http` connector exports canonical numeric events as OTLP gauge metrics to `/v1/metrics` and non-metric events as OTLP log records to `/v1/logs`.

The gateway accepts `POST /v1/logs` and `POST /v1/metrics` with `Content-Type: application/json` under the existing `ingest` scope. Logs preserve resource/service identity, attributes, trace/span IDs, severity, schema URL, and supported AnyValue bodies. Metrics accept numeric gauge and sum datapoints; unsupported forms are reported through OTLP JSON `partialSuccess`.

OTLP protobuf ingestion is explicitly unsupported in v1.8 and returns HTTP 415.
