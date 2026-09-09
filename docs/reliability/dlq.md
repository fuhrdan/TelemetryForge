# Dead-Letter Queue

The dead-letter queue is `telemetry.dlq`.

A source Kafka record is moved there when:

- the payload cannot be decoded;
- a permanent processing error occurs; or
- a transient error exhausts its local retry attempts.

## Dead-letter envelope

Each DLQ record contains:

```text
event                  decoded canonical event, when available
raw_payload_base64     malformed bytes, when decoding failed
original_topic
partition
offset
failure_class
error
attempts
failed_at
```

The original topic/partition/offset are preserved because "this failed" is not
enough information during an incident. Operators need to know exactly where the
record came from.

## Acknowledgement ordering

```text
processing fails permanently
        |
        v
publish DLQ record
        |
        +-- DLQ publish fails --> leave source offset uncommitted
        |
        v
DLQ acknowledged by Kafka
        |
        v
commit source offset
```

This means TelemetryForge does not discard the original record until Kafka has
durably accepted its dead-letter replacement.

## Replay

The `telemetryctl` replay path is:

```bash
telemetryctl dlq replay --file dead-letter.json
```

By default, the embedded event is sent back to its original topic. A topic can
be overridden explicitly.

Malformed raw payloads cannot be automatically replayed because doing so would
knowingly inject invalid JSON back into the normal pipeline.

Later releases will add richer DLQ browsing and policy-aware replay.
