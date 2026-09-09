# Kafka Partitioning

TelemetryForge keys ingested Kafka records by canonical event `source`.

```text
checkout-api ----+
checkout-api ----+--> hash(source) --> partition 2
checkout-api ----+

payment-api --------------------> partition 4
inventory-api ------------------> partition 1
```

## Why source is the key

This gives two useful properties:

1. events from one source retain Kafka partition order; and
2. independent sources can be spread across partitions and processed in
   parallel.

The choice is intentionally simple. A future multi-tenant deployment may use a
compound tenant/tenant/source key to avoid letting one tenant's source naming affect
another tenant's ordering domain.

## Consumer-group scale

The useful concurrency of one consumer group is bounded by partition count.

With six partitions, at most six consumers in the same group can actively own
partitions for one six-partition topic at a time. Adding a seventh worker
replica does not create a seventh active partition owner.

Within one process, the bounded Go worker pool controls processing concurrency.
Across processes, Kafka partition ownership controls group concurrency.

These are different scaling knobs and v0.7.0 Kubernetes documentation should
make that distinction visible.

## Ordering versus concurrent processing

Partition ordering at Kafka does not automatically mean processors finish in
order. TelemetryForge's worker pool can complete later offsets first.

The acknowledgement coordinator preserves correctness by committing only the
contiguous completed prefix for each partition. See
`docs/kafka/delivery-semantics.md`.
