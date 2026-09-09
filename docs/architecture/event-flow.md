# Event Flow — v0.3.0

1. A client sends an event or metric to the gateway.
2. The gateway validates and canonicalizes the envelope.
3. Kafka acknowledges the published record.
4. The gateway returns HTTP `202 Accepted`.
5. A member of `telemetryforge-processors` receives the record.
6. The consumer decodes it into the canonical Go domain type.
7. The record waits in the bounded worker queue if all workers are busy.
8. A worker runs the processing pipeline.
9. On success, the Kafka offset is explicitly committed.
10. On processing failure, the record is not acknowledged.

This creates two useful guarantees:

- the gateway does not claim durable acceptance before Kafka acknowledges;
- the worker does not claim successful consumption before processing succeeds.

The overall processing model is at least once, so later database writes must be
idempotent.
