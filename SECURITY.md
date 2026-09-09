# Security Policy

## v0.5.0 security posture

TelemetryForge v0.5.0 is a development/portfolio release. Its reliability model
is substantially stronger, but it is **not yet a safe public multi-tenant
service**.

Not yet implemented:

- ingestion authentication
- tenant isolation and authorization
- application TLS termination
- Kafka TLS/SASL
- rate limiting
- secrets management
- PII classification/redaction
- per-tenant retention policy

## DLQ and Flight Recorder sensitivity

The new reliability features intentionally preserve failed/full-fidelity data.

That means:

- `telemetry.dlq` can contain original event bodies;
- malformed payload bytes are preserved;
- `flight_recorder_events` stores pre-normalized envelopes;
- frozen `incident_events` can outlive the rolling buffer.

Operators must treat these locations with the same or greater sensitivity as
the primary telemetry datastore.

A future policy layer will redact or quarantine sensitive fields before
crossing trust boundaries. v0.5.0 does not provide that guarantee.

## Local credentials

Credentials in `docker-compose.yml` are deliberately obvious local-development
credentials. They must never be reused in a real deployment.

**Do not expose v0.5.0 directly to an untrusted network or the public Internet.**
