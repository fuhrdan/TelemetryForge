# Producer Delivery Semantics

## HTTP acceptance contract

TelemetryForge does **not** return `202 Accepted` merely because a request passed validation.

The sequence is:

```text
HTTP request
    ↓
validate canonical envelope
    ↓
generate event ID if needed
    ↓
serialize JSON
    ↓
produce to Kafka
    ↓
wait for broker acknowledgement
    ↓
HTTP 202 Accepted
```

If Kafka cannot acknowledge the event before the producer timeout, the gateway returns `503 Service Unavailable`.

## Acknowledgements

The producer requests acknowledgements from all in-sync replicas. The local single-broker development environment has only one replica, while production deployments are expected to use a replicated topic configuration.

## Idempotence

The franz-go producer keeps its built-in idempotent producer behavior enabled. This reduces duplicates caused by retries at the Kafka producer protocol layer.

Application-level end-to-end idempotency is intentionally deferred until the reliability milestone because downstream consumers and persistence do not yet exist.

## Delivery guarantee

v0.2.0 should be understood as a **durable producer boundary**, not a full end-to-end delivery guarantee. End-to-end at-least-once processing semantics will be defined when consumers, offset commits, retries, and persistence are implemented.
