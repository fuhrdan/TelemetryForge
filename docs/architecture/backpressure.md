# Backpressure

Backpressure means a slow downstream component is allowed to slow the component
feeding it instead of causing unlimited buffering.

TelemetryForge has three relevant buffers:

1. Kafka, which is durable and designed to hold backlog.
2. The consumer's fetched records.
3. The worker pool's bounded Go channel.

The third buffer is intentionally small and configurable. When it fills,
`Pool.Submit` waits. The Kafka consumer therefore stops advancing quickly and
consumer lag grows in Kafka rather than memory usage growing without limit.

## Configuration

```text
TELEMETRYFORGE_WORKER_COUNT=4
TELEMETRYFORGE_WORKER_QUEUE_CAPACITY=256
```

More workers increase parallel processing. A larger queue can smooth short
bursts but should not be treated as a substitute for adequate processing
capacity.

## Operational interpretation

A growing Kafka lag with stable worker memory means backpressure is working.
A permanently growing lag means processing capacity is insufficient and the
worker deployment should be scaled or the downstream bottleneck investigated.
