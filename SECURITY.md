# Security Policy

## Project maturity

TelemetryForge is currently pre-1.0. The stable v0.6.0 release and v0.7.0
development branch demonstrate reliability and observability architecture, but
they do **not** yet implement the complete security boundary required for a
public multi-tenant telemetry service.

## Not yet implemented

- ingestion/dashboard authentication
- tenant isolation and authorization
- application TLS termination policy
- Kafka TLS/SASL defaults
- production secret management
- source/tenant-specific PII redaction
- per-tenant retention and access controls
- hardened Internet-facing browser security policy

## Sensitive data locations

Treat all of these as potentially sensitive:

- primary `telemetry_events` payload/tag data
- `telemetry.dlq`
- malformed raw payload bytes stored in dead-letter records
- `flight_recorder_events`
- durable `incident_events`

The Flight Recorder intentionally preserves the pre-normalized envelope, so it
may contain fields that later processors would otherwise remove.

## Local credentials

Credentials in `.env.example` and `docker-compose.yml` are obvious
local-development defaults. They must never be reused in a real deployment.

## Reporting a vulnerability

Do not post exploit details, credentials, or real private telemetry in a public
issue.

Prefer GitHub private vulnerability reporting / a Security Advisory if it is
enabled for the repository. If private reporting is unavailable, open a
minimal public issue stating that you have a security concern and request a
private contact path without including the exploit details.

Useful information in a private report includes:

- affected version/commit
- affected component
- reproduction steps using synthetic data
- impact
- suggested mitigation, if known

## Supported versions

Because the project is pre-1.0, security fixes are expected to target the
latest development/stable line rather than maintaining a long-lived patch
matrix for old milestones.

**Do not expose the current Docker Compose stack directly to an untrusted
network or the public Internet.**
