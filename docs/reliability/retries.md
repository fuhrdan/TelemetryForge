# Retry Policy

TelemetryForge uses explicit retry classification instead of retrying every error
the same way.

## Failure classes

### Transient

A transient failure may succeed if attempted again.

Examples:

- database temporarily unavailable
- short network interruption
- dependency timeout

Transient work uses bounded exponential backoff.

### Permanent

Repeating the same input is not expected to help.

Examples:

- malformed event data
- normalization/validation failure
- unsupported payload shape

Permanent failures skip local retries and move directly to the dead-letter path.

Unknown errors default to **permanent**. This conservative choice prevents
programming bugs from becoming endless retry loops.

## Default retry schedule

The worker allows four total attempts.

```text
attempt 1
  |
  +-- transient failure
  |
  v
~250 ms + jitter
attempt 2
  |
  v
~500 ms + jitter
attempt 3
  |
  v
~1 s + jitter
attempt 4
  |
  v
DLQ if still failing
```

Delay is exponential, capped, and includes 20% jitter so many workers recovering
from the same dependency outage do not all retry at exactly the same instant.

## Why keep retries local and bounded?

Kafka is already the durable backlog. TelemetryForge should not create a second
unbounded retry queue in process memory.

Local retry exists only to absorb short-lived failures. Persistent failures
become explicit DLQ records that can be inspected and replayed.
