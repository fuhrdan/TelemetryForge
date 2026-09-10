# Connector Platform

TelemetryForge v1.8 separates routing policy from backend protocol translation. The Telemetry Router still owns durable outbox state, retries, DLQ, fallback, and leases. Connectors own only protocol translation, client initialization, capabilities, one-event delivery, backend readiness, and retryable/permanent failure classification.

Built-in kinds: `kafka`, `http_json`, `otlp_http`, `prometheus_remote_write`, `splunk_hec`, and `datadog_logs`.

Legacy `type: kafka` and `type: http` routing documents remain valid and are translated into the connector contract. New documents should use `type: connector` with nested `connector.kind`.

Connector runtime state contains no secrets. Router instances probe destinations every 30 seconds; only two-minute-fresh state is shown live, and stale rows are pruned after 24 hours. An unhealthy connector does not make the router process unready because destination isolation is a core design goal.
