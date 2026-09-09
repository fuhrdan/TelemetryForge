# TelemetryForge v1.1.0 Release Notes

## Schema Intelligence

v1.1.0 adds a versioned, tenant-aware schema registry that learns from real
telemetry after normalization and before policy mutation.

The feature is advisory by design: schema drift is explained and surfaced, not
used as an automatic ingestion rejection mechanism.

## Registry model

Each schema is keyed by:

```text
tenant_id
source
event_type
schema_version
```

The canonical envelope also accepts optional:

```text
schema_url
```

for the OpenTelemetry semantic-convention schema identifier.

Application `schema_version` and OpenTelemetry `schema_url` remain distinct.

## Drift detection

v1.1 records:

- additive fields as `info`;
- same-version JSON type changes as `breaking`;
- established required-field disappearance as `breaking`;
- legacy semantic-convention names as `warning`;
- changed `schema_url` under the same application version as `warning`;
- version-to-version field additions/removals/type changes.

A field becomes inferred required only after at least 20 observations and at
least 95% presence. This avoids treating sparse optional fields as mandatory.

## Bounded inspection

Per event, Schema Intelligence is bounded to:

```text
512 fields
5 payload nesting levels
```

Oversized shapes create a `schema_truncated` warning and continue through the
normal telemetry pipeline.

Accumulated state is separately bounded to 2,048 unique field paths per
tenant/source/type/version. Additional dynamic paths stop accumulating and
produce a deduplicated `registry_field_limit` warning rather than growing the
registry indefinitely.

## Retry / failure safety

Schema observations use:

```text
PRIMARY KEY (tenant_id, event_id)
```

so Kafka retry cannot inflate counts.

The per-event observation ledger can be pruned with
`telemetryctl schema prune`; the conservative default is 35 days and does not
delete accumulated registry/drift history.

Registry dependency failures are fail-open by default:

```text
TELEMETRYFORGE_SCHEMA_FAIL_OPEN=true
```

An operator may explicitly choose fail-closed behavior.

## OpenTelemetry awareness

The built-in compatibility subset recognizes high-value stable attributes such
as:

```text
service.name
deployment.environment.name
http.request.method
http.response.status_code
network.protocol.version
server.address
server.port
url.scheme
```

It also warns on common legacy names including `http.method` and
`http.status_code`.

The focused catalog was aligned with OpenTelemetry Semantic Conventions 1.44.0
at release-build time; it is intentionally not a bundled copy of the full
upstream registry.

## API

Added:

```text
GET /api/v1/schemas
GET /api/v1/schema-history?source=...&type=...
GET /api/v1/schema-drift
GET /api/v1/schema-diff?source=...&type=...&from=...&to=...
```

All endpoints inherit the v1 authenticated tenant boundary.

## CLI

Added:

```bash
telemetryctl schema inspect --source orders-api --type order.created
telemetryctl schema diff --source orders-api --type order.created --from 1.0 --to 2.0
telemetryctl schema prune --older-than 840h
```

## Dashboard

Added a **Schema Intelligence** surface showing:

- current schema health counts;
- latest source/type/version registry entries;
- field and inferred-required counts;
- selectable schema version history; and
- recent drift findings.

## Demo

```bash
make demo-schema
```

creates an established schema, same-version drift, a legacy semantic attribute,
and a deliberately breaking v2 declaration.

## Database

Migration `008_schema_intelligence.sql` adds:

- `schema_observations`
- `schema_registry`
- `schema_drift_findings`

## Security / compatibility

The v1.0 authentication, tenant-isolation, dashboard proxy, redaction, Kafka
security, and production deployment boundaries remain in place.

Schema registry reads and writes are tenant-scoped.
