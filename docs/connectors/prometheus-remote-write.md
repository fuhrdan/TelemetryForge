# Prometheus Remote Write Connector

The `prometheus_remote_write` connector exports canonical numeric events as one-sample Prometheus time series using Remote Write 0.1, protobuf, and Snappy encoding.

Always-present labels are `__name__`, `source`, and `tenant_id`. Telemetry tags are not automatically converted to labels. Operators must explicitly allow reviewed tags through `prometheus_label_tags`, capped at 32 names, to avoid recreating uncontrolled downstream cardinality.

A non-numeric event is a permanent connector rejection and goes directly to the router's destination DLQ/fallback path rather than consuming retries.
