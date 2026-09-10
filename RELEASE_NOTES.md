# TelemetryForge v1.8.0 Development Release Notes

## Connector Platform

v1.8 separates backend protocol translation from durable routing state. The router continues to own outbox leasing, retries, destination DLQ, fallback, and completion. Connectors own protocol translation, readiness, capabilities, and retryable/permanent error classification.

Built-ins: Kafka, generic HTTP JSON, OTLP/HTTP JSON, Prometheus Remote Write 0.1, Splunk HEC, and Datadog Logs. OTLP/HTTP JSON ingestion is available at `/v1/logs` and `/v1/metrics`; protobuf ingestion is explicitly unsupported. Connector secrets are environment-backed. Runtime connector heartbeats are exposed through API/dashboard and `telemetryforge_connector_ready`.
