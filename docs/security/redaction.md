# API Redaction

TelemetryForge's Flight Recorder exists to preserve incident evidence. Destroying
sensitive values before storage can conflict with that purpose.

v1.0 therefore adds **presentation-time redaction**.

## Tag redaction

Configure a comma-separated list:

```text
TELEMETRYFORGE_REDACT_TAG_KEYS=
  authorization,cookie,email,user_email,token,password,api_key
```

Matching tag values returned from APIs/SSE are replaced with:

```text
[REDACTED]
```

## Payload redaction

Production can enable:

```text
TELEMETRYFORGE_REDACT_PAYLOAD=true
```

API/dashboard/SSE responses then receive:

```json
{"redacted": true}
```

instead of the original payload.

## What is not changed

Redaction does not mutate:

- Kafka records already accepted;
- primary stored telemetry;
- Flight Recorder evidence;
- frozen incident evidence;
- quarantine evidence.

This is intentional. v1.0 treats redaction as a read/export boundary while
preserving the forensic source of truth.

## Security consequence

Full-fidelity storage remains sensitive.

Production access controls must protect PostgreSQL/TimescaleDB backups,
administrative SQL access, and any direct data export path.

A future deployment may add tenant-specific destructive ingestion redaction for
organizations whose policy forbids storing specific data at all.
