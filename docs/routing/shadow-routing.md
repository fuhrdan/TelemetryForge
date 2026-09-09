# Shadow Routing

Shadow routing lets operators test a candidate destination policy against real
post-policy telemetry without sending traffic to candidate destinations.

## Production invariant

The active routing configuration creates durable outbox rows.

The shadow configuration creates **only a destination-set comparison**.

It cannot:

- enqueue a candidate destination delivery;
- call a webhook;
- publish to a candidate Kafka topic; or
- trigger a failure fallback.

## What is recorded

For an event where active and candidate routing differ, TelemetryForge stores:

```text
active config/version
shadow config/version
active destination set
shadow destination set
added destinations
removed destinations
```

The event payload is not duplicated into the shadow-diff table.

## Example

Active:

```text
checkout error -> primary + security
```

Candidate:

```text
checkout error -> security
```

Shadow diff:

```text
added:   []
removed: [primary]
```

This makes routing changes reviewable before promotion.

## Dashboard

The **Shadow Routing** panel shows recent added/removed destination decisions.

This is intentionally separate from the Cardinality Firewall's shadow-policy
panel because telemetry mutation and telemetry destination selection are two
different operational decisions.
