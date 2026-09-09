# telemetryctl

`telemetryctl` is the small operational CLI introduced in v0.5.0.

It intentionally starts with a few explicit commands rather than becoming a
large administration shell before the operational model is mature.

## Freeze an incident

```bash
telemetryctl incident freeze \
  --id INC-2026-0042 \
  --title "Checkout latency spike" \
  --from 2026-09-09T16:00:00Z \
  --to 2026-09-09T16:20:00Z
```

This copies matching records from the 30-minute rolling Flight Recorder into
durable incident storage.

## Replay an exported DLQ event

```bash
telemetryctl dlq replay --file dead-letter.json
```

The event goes back to its original Kafka topic unless `--topic` overrides it.

A malformed raw payload is not replayed automatically because TelemetryForge
cannot safely turn invalid JSON into a valid canonical event.

## Prune old idempotency reservations

```bash
telemetryctl dedup prune
```

Default horizon: 35 days.

The command refuses horizons under 30 days in v0.5.0. Read
`docs/reliability/dedup-lifecycle.md` before changing the value.

## Environment

The CLI understands:

```text
TELEMETRYFORGE_DATABASE_URL
TELEMETRYFORGE_KAFKA_BROKERS
```

Flags override environment values where provided.
