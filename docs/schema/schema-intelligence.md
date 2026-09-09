# Schema Intelligence

TelemetryForge v1.1.0 adds a tenant-scoped schema registry and drift detector
for the telemetry flowing through the control plane.

The goal is not to reject every unfamiliar event. The goal is to answer:

- What shape does this source/type normally emit?
- Which fields are stable enough to be treated as required?
- Did a field change type without a declared version change?
- Did a new declared version remove or change an established field?
- Is the producer still using a legacy OpenTelemetry semantic attribute?
- Which schema versions have actually appeared in production telemetry?

## Pipeline position

Schema Intelligence runs:

```text
Flight Recorder
    -> Normalizer
    -> Schema Intelligence
    -> Cardinality Firewall
    -> Persistence
    -> Incident Detector
```

This ordering is deliberate.

The Flight Recorder still captures the original decoded event first. Schema
Intelligence then observes the normalized event **before policy mutates it**.

A `drop_tag` policy can therefore protect downstream telemetry without hiding
that the application originally emitted the field.

## Identity

A registry entry is identified by:

```text
tenant_id
source
event_type
declared schema_version
```

`schema_version` is the application's version identifier from the canonical
TelemetryForge envelope.

OpenTelemetry's optional `schema_url` is stored separately. It identifies the
OpenTelemetry semantic-convention schema associated with telemetry and is not a
replacement for an application's own schema version.

## What becomes a field?

Schema Intelligence tracks:

- `metric.value`
- `metric.unit`
- `tags.<key>`
- JSON payload paths such as `payload.customer.id`

Payload objects are walked deterministically.

Arrays are represented as an `array` field rather than recursively expanding
every element. This prevents untrusted payload contents from turning schema
inspection into unbounded state.

## Bounds

One event can contribute at most:

```text
512 fields
5 nested payload levels
```

If a shape exceeds those limits, the event still continues through the normal
pipeline and Schema Intelligence records a `schema_truncated` warning.

## Idempotent observations

Worker retries must not inflate schema counts.

`schema_observations` uses:

```text
PRIMARY KEY (tenant_id, event_id)
```

so the same at-least-once Kafka delivery is counted once.

## Additive drift

If a field appears for the first time under an already-observed
`schema_version`, TelemetryForge records:

```text
kind: field_added
severity: info
```

This is useful evidence, but it is not automatically treated as a breaking
change.

## Type drift

If an established field changes JSON type without an application version
change, TelemetryForge records:

```text
kind: type_changed
severity: breaking
```

Example:

```text
payload.total: number
        ->
payload.total: string
```

The registry retains the original expected type and increments a conflict
counter instead of silently redefining the old schema.

## Required-field inference

A single event cannot reliably tell TelemetryForge whether a field is optional.

A field becomes inferred `required` only after:

```text
at least 20 observations
and
presence in at least 95% of them
```

After that point, a missing field becomes:

```text
kind: required_field_missing
severity: breaking
```

This avoids turning sparse optional payloads into constant false alarms.

## Declared version changes

The first event for a new `schema_version` is compared against the most recently
observed prior version for the same tenant/source/type.

Version diff rules:

| Change | Default classification |
|---|---|
| field added | compatible / info |
| optional field removed | review / warning |
| inferred-required field removed | breaking |
| field type changed | breaking |

The comparison is evidence about compatibility. TelemetryForge v1.1 does not
block the new version automatically.

## Health

A registry version has one of:

```text
healthy
warning
breaking
```

Health is intentionally sticky within one declared version. If that version has
produced a breaking type conflict, later healthy events do not erase the fact
that the version has violated its own established shape.

A new declared version creates a new health record.

## Failure behavior

Schema Intelligence is **fail-open by default**:

```text
TELEMETRYFORGE_SCHEMA_FAIL_OPEN=true
```

If registry persistence is temporarily unavailable, the worker logs the
failure and continues normal telemetry processing.

This feature is advisory and must not become a new production ingestion outage.

An operator can explicitly choose fail-closed behavior:

```text
TELEMETRYFORGE_SCHEMA_FAIL_OPEN=false
```

In that mode registry dependency errors use the normal bounded transient retry
path.

## API

Current tenant schemas:

```text
GET /api/v1/schemas
```

History for one source/type:

```text
GET /api/v1/schema-history?source=orders-api&type=order.created
```

Recent drift:

```text
GET /api/v1/schema-drift
```

Compare two stored declared versions:

```text
GET /api/v1/schema-diff?source=orders-api&type=order.created&from=1.0&to=2.0
```

All reads inherit the authenticated tenant boundary from v1.0.0.

## CLI

Inspect history:

```bash
telemetryctl schema inspect \
  --source orders-api \
  --type order.created \
  --tenant default
```

Compare versions:

```bash
telemetryctl schema diff \
  --source orders-api \
  --type order.created \
  --from 1.0 \
  --to 2.0 \
  --tenant default
```

## Demo

```bash
make demo-schema
```

The demo deliberately creates:

1. an established v1 schema;
2. same-version missing/type drift;
3. a legacy OpenTelemetry HTTP attribute; and
4. a declared v2 breaking change.

Refresh the dashboard and inspect **Schema Intelligence**.


## Observation-ledger maintenance

`schema_observations` is an idempotency ledger, not the schema history itself.
It stores one small row per observed event so a retry cannot inflate registry
counts or manufacture drift.

Prune old ledger reservations with:

```bash
telemetryctl schema prune --older-than 840h
```

The default is 35 days. The CLI refuses horizons shorter than 30 days so the
ledger normally outlives the development telemetry/incident investigation
window.

Pruning this table does **not** delete:

- accumulated schema versions;
- inferred field state;
- semantic findings; or
- drift history.
