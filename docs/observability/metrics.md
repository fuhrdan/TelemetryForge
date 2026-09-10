# Prometheus Metrics Catalog

All custom metrics use the `telemetryforge_` prefix and a constant `service`
label (`gateway` or `worker`).

## Gateway

### `telemetryforge_http_requests_total`

Labels:

```text
service
method
route
status
```

`route` is a registered HTTP pattern, never the raw path.

### `telemetryforge_http_request_duration_seconds`

Histogram labels:

```text
service
method
route
```

### `telemetryforge_accepted_events_total`

Labels:

```text
service
kind
topic
```

### `telemetryforge_kafka_publish_failures_total`

Labels:

```text
service
topic
```

## Worker

### `telemetryforge_worker_jobs_total`

Label:

```text
outcome
```

Current outcomes include:

```text
success
dlq
ack_failed
failed
```

### `telemetryforge_worker_job_duration_seconds`

Histogram of end-to-end worker processing time.

### `telemetryforge_worker_retries_total`

Label:

```text
classification
```

### `telemetryforge_worker_queue_depth`

Current bounded queue depth.

### `telemetryforge_worker_queue_capacity`

Configured bounded queue capacity.

### `telemetryforge_kafka_consumer_lag`

Labels:

```text
topic
partition
```

This is broker-derived group lag, not local queue depth or last-fetch size.

### `telemetryforge_kafka_consumer_lag_total`

Sum of current non-negative topic/partition lag for the processing group.

### `telemetryforge_dead_letters_total`

Label:

```text
classification
```

## Runtime collectors

Each process registry also exposes Go runtime and process collectors.

## Cardinality rules for future metrics

Before adding a label, ask:

> Can an untrusted or high-volume telemetry field create a new label value?

If yes, do not use it as a Prometheus label.

Good dimensions:

```text
route pattern
status class/value
topic
partition
bounded outcome
bounded classification
```

Bad dimensions:

```text
event ID
incident ID
correlation ID
request ID
arbitrary source
arbitrary tag
raw URL
error text
```


## Router

### `telemetryforge_routing_deliveries_total`

Labels:

```text
service=router
destination
outcome
```

Current outcomes include:

```text
delivered
retry
dead_letter
fallback_enqueued
```

Destination names are bounded operator configuration values, not telemetry
values. They are therefore acceptable Prometheus labels under the project's
cardinality rules.


## Adaptive sampling / shaping

### `telemetryforge_shaping_decisions_total`

Labels:

```text
service
rule
outcome
```

`rule` comes only from the validated shaping document; configuration caps the
number and length of rule names. `outcome` is bounded to `kept`, `protected`,
or `sampled_out`.

### `telemetryforge_shaping_queue_pressure_ratio`

Current process-local bounded worker queue utilization used by pressure-aware
sampling. The value is between 0 and 1.

These metrics describe policy decisions. They are separate from downstream
persistence/routing success metrics.

## Connector Platform

`telemetryforge_connector_ready{service="router",destination,kind}` is 1 when the latest connector readiness probe succeeds and 0 when it fails. Connector readiness is intentionally separate from router process readiness.
