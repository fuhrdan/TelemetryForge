# Kafka Topic Strategy

## v0.2.0 topics

| Topic | Purpose | Default partitions | Local replication |
|---|---|---:|---:|
| `telemetry.raw` | Generic logs, webhooks, traces, and custom events | 6 | 1 |
| `telemetry.metrics` | Numeric metric events | 6 | 1 |

The single-broker local environment necessarily uses replication factor 1. Production deployments should use multiple brokers and a replication factor appropriate to the availability target.

## Why separate raw events and metrics?

Metrics and generic events are expected to diverge in processing, retention, and aggregation behavior. Separating the topics now prevents later worker pipelines from requiring expensive event-type filtering before they can scale independently.

## Reserved future topics

Later milestones are expected to introduce topics such as:

- `telemetry.dlq`
- `telemetry.replay`
- `telemetry.incident`
- `telemetry.shadow`

They are intentionally not created in v0.2.0 because their semantics are not implemented yet.
