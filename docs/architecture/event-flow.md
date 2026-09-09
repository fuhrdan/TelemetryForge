# Event Flow — v0.2.0

```mermaid
sequenceDiagram
    participant C as Client
    participant G as Gateway
    participant V as Validator
    participant K as Kafka

    C->>G: POST /api/v1/events
    G->>V: Validate envelope
    V-->>G: Valid
    G->>G: Generate event ID
    G->>K: Produce telemetry.raw
    K-->>G: Broker acknowledgement
    G-->>C: 202 Accepted
```

If Kafka is unavailable or the produce operation exceeds the configured timeout, the gateway returns `503 Service Unavailable` and does not claim the event was accepted.

This behavior is intentional: the HTTP boundary should not lie about durability.
