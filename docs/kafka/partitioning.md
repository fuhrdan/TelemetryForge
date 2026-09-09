# Kafka Partitioning

TelemetryForge v0.2.0 uses the event `source` as the Kafka record key.

```text
checkout-api ─┐
checkout-api ─┼── hash(source) ──> partition 2
checkout-api ─┘

payment-api ────────────────> partition 4
inventory-api ──────────────> partition 1
```

This provides two useful properties:

1. Events from one source retain partition-local order.
2. Independent sources can be processed concurrently across partitions.

## Scale implication

The useful concurrency of a future consumer group is bounded by the number of partitions. With six partitions, at most six consumers in the same group can actively own partitions for a topic at one time.

Consumer-group workers now use this partitioning model. Partition count bounds useful group-level concurrency, while the bounded worker queue controls in-process concurrency and backpressure.
