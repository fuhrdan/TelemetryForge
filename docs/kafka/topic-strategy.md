# Kafka Topic Strategy

## Current topics

| Topic | Purpose | Local partitions | Local replication |
|---|---|---:|---:|
| `telemetry.raw` | Generic logs, traces, webhooks, and custom events | 6 | 1 |
| `telemetry.metrics` | Numeric metric events | 6 | 1 |
| `telemetry.dlq` | Terminal failures and malformed source records | 6 | 1 |

The local environment uses one Kafka broker, so replication factor 1 is the
only meaningful development setting. Production deployments should use
multiple brokers and an availability-appropriate replication factor.

## Why separate raw events and metrics?

Metrics and generic events have different likely aggregation, retention, and
query behavior. Separate topics let processing paths evolve independently
without forcing every consumer to inspect and discard unrelated records.

## Dead-letter topic

`telemetry.dlq` is an operational backlog, not a trash can.

A dead-letter envelope preserves:

- the decoded canonical event when possible;
- base64-encoded raw bytes for malformed records;
- original topic, partition, and offset;
- failure classification;
- error message;
- attempt count; and
- failure timestamp.

The original source offset advances only after Kafka acknowledges the DLQ
replacement.

## Planned topics

Future capabilities may justify separate topics such as:

- `telemetry.replay`
- `telemetry.incident`
- `telemetry.shadow`

They should not be created until their delivery, retention, and ownership
semantics are implemented and documented. Creating speculative topics early
would make the architecture look more complete than it actually is.


## v1 tenant-aware partition key

Normal telemetry records use:

```text
tenant_id | source
```

as the Kafka record key.

This preserves ordering for one source inside one tenant while keeping two
tenants with the same service name logically distinct.
