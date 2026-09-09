# TelemetryForge Architecture — v0.5.0

TelemetryForge currently has two independently scalable runtime services plus
two durable infrastructure systems.

```text
                    +----------------------+
HTTP / webhooks --->| Go ingestion gateway |
                    +----------+-----------+
                               |
                               v
                         Apache Kafka
                               |
                               v
                    telemetryforge-processors
                               |
                               v
                       bounded worker pool
                               |
                +--------------+--------------+
                |                             |
                v                             v
        Flight Recorder                 processing chain
        rolling buffer                       |
                |                             v
                |                        TimescaleDB
                |                             |
                +--> frozen incidents         +--> Query API

terminal failures -----------------------> telemetry.dlq
```

## Ingestion tier

The gateway validates the canonical event envelope and waits for Kafka
acknowledgement before returning HTTP `202 Accepted`.

## Processing tier

The worker uses consumer groups, a bounded queue, classified retries, the
Flight Recorder, normalization, and idempotent persistence.

A source offset is committed only after either:

- normal persistence succeeds; or
- a terminal failure is durably published to `telemetry.dlq`.

## Storage tier

PostgreSQL/TimescaleDB provides:

- time-series telemetry storage
- event-ID deduplication
- short-lived Flight Recorder storage
- durable frozen incident windows

## Why separate Kafka from storage?

Kafka absorbs bursty producer traffic and decouples ingestion availability from
database processing speed. TimescaleDB provides queryable analytical storage.
Each system does the job it is designed for instead of using application memory
as the buffer between them.

## Where the project goes next

v0.6.0 can now build a real-time dashboard and automatic incident capture on a
reliable ingest/process/store foundation.
