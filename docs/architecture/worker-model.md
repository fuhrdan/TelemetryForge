# Worker Model

## v0.4.0 processing path

The gateway writes accepted telemetry to Kafka. A separate `worker` process
joins the `telemetryforge-processors` consumer group and persists records into
PostgreSQL/TimescaleDB.

```text
Kafka
  |
  v
Consumer group
  |
  v
Bounded in-memory queue
  |
  +---- Worker 1
  +---- Worker 2
  +---- Worker N
          |
          v
     Normalizer
          |
          v
      Persister
          |
          v
 PostgreSQL/TimescaleDB
          |
          v
   Kafka offset commit
```

## Why a bounded queue?

The queue is deliberately finite. If persistence slows, an unlimited Go
channel would hide the problem until the process exhausted memory. A full
bounded queue instead slows consumption so backlog stays in Kafka, the durable
system designed to hold it.

## Why persist before committing the Kafka offset?

If the worker acknowledged Kafka first and then the database write failed,
telemetry could be lost permanently.

v0.4.0 therefore acknowledges the Kafka record only after the database
transaction succeeds. This gives at-least-once processing.

## What about duplicates?

At-least-once processing means a record can be processed twice after a crash or
rebalance. The persistence layer makes that retry safe by reserving each
canonical event ID in a globally unique PostgreSQL table before inserting the
time-series row.

## Processor pipeline

Processors remain independent of Kafka transport. v0.4.0 composes:

1. `Normalizer`
2. `Persister`

That separation is intentional. Future schema validation, enrichment,
Cardinality Firewall rules, incident capture, and replay can be inserted into
the pipeline without rewriting the consumer.
