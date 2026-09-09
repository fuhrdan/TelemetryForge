# Worker Model

## v0.5.0 processing path

The worker is now responsible for three distinct guarantees:

1. keep memory bounded;
2. preserve short-lived full-fidelity incident evidence;
3. either persist an event successfully or move it to a durable DLQ.

```text
Kafka source record
       |
       v
bounded worker queue
       |
       v
Flight Recorder
       |
       v
Normalizer
       |
       v
TimescaleDB persistence
       |
       +---- success -----------------> commit source offset
       |
       `---- failure
               |
               +-- transient --> bounded retry
               |                   |
               |                   `-- exhausted
               |
               `-- permanent
                       |
                       v
                 telemetry.dlq
                       |
                       v
                commit source offset
```

## Why record before normalization?

The Flight Recorder exists to preserve evidence. It should contain what the
pipeline received, not only what later processing decided to keep or change.

## Why retry inside a worker slot?

A retry remains attached to the worker that owns the job. That keeps retry
concurrency bounded by worker count. TelemetryForge does not launch a new
goroutine or allocate a new queue for every failed attempt.

## Why does DLQ success count as completion?

Once Kafka durably contains a detailed dead-letter replacement, the poison
record no longer needs to block the source partition. The DLQ preserves the
event, source location, failure reason, and attempt count for investigation and
replay.

If publishing that replacement fails, the original source offset remains
uncommitted.
