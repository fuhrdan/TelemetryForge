# telemetryctl

`telemetryctl` is the operational CLI for incident capture, DLQ replay, and deduplication maintenance.

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

The command refuses horizons under 30 days. Read
`docs/reliability/dedup-lifecycle.md` before changing the value.

## Environment

The CLI understands:

```text
TELEMETRYFORGE_DATABASE_URL
TELEMETRYFORGE_KAFKA_BROKERS
```

Flags override environment values where provided.


## Validate policy

```bash
telemetryctl policy validate --file policies/active.json
```

This performs strict JSON/schema validation without starting a worker.

Use it for both active and shadow files before deployment.


## Replay a frozen incident

Analysis-only:

```bash
telemetryctl incident replay \
  --id INC-2026-0042 \
  --policy policies/active.json \
  --shadow-policy policies/shadow.json
```

Optional isolated output:

```bash
telemetryctl incident replay \
  --id INC-2026-0042 \
  --publish-topic telemetry.replay.candidate-policy
```

Production topics are rejected.

## Simulate telemetry cost/shape

Without pricing:

```bash
telemetryctl cost simulate \
  --incident INC-2026-0042
```

With explicit pricing:

```bash
telemetryctl cost simulate \
  --incident INC-2026-0042 \
  --pricing pricing/my-reviewed-model.json
```

No pricing file means no dollar estimate.


## Inspect schema history

```bash
telemetryctl schema inspect   --source orders-api   --type order.created   --tenant default
```

The output includes each observed application-declared version, health,
observation counts, inferred-required fields, semantic findings, and schema
fingerprint.

## Diff schema versions

```bash
telemetryctl schema diff   --source orders-api   --type order.created   --from 1.0   --to 2.0   --tenant default
```

Compatibility is `compatible`, `review`, or `breaking` based on field additions,
removals, inferred-required state, and type changes.


## Prune old schema observation reservations

Schema Intelligence uses a per-event observation ledger to make worker retries
idempotent. Remove reservations older than the normal investigation horizon:

```bash
telemetryctl schema prune --older-than 840h --tenant default
```

Default: 35 days. The command refuses values below 30 days.

This maintenance command removes only the idempotency ledger. Registry versions,
field state, and drift findings remain intact.


## Inspect distributed cardinality

Top active dimensions:

```bash
telemetryctl cardinality top \
  --mode active \
  --limit 25 \
  --tenant default
```

Candidate/shadow state:

```bash
telemetryctl cardinality top --mode shadow
```

The result is the current shared one-hour HLL view, not one worker's local
memory.

## Inspect series budgets

```bash
telemetryctl cardinality budgets --tenant default
```

Budget output is advisory and includes the policy name/version/mode that
produced the status.
