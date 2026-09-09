# Security Policy

## v0.6.0 security posture

TelemetryForge v0.6.0 adds a browser dashboard and automatic incident capture,
but it remains a development/portfolio release rather than a public multi-tenant
service.

Not yet implemented:

- dashboard or ingestion authentication
- tenant isolation / authorization
- application TLS termination
- Kafka TLS/SASL
- production secrets management
- PII classification/redaction
- per-tenant retention
- browser security headers tuned for Internet exposure

## Dashboard exposure

The dashboard can read stored telemetry, live event data, frozen incidents, and
trigger reasons. Treat it as an administrative surface.

The Next.js application proxies the Go API on the same browser origin. This
simplifies local development and avoids opening broad CORS access, but it does
not replace authentication.

## Flight Recorder sensitivity

The Flight Recorder and incident archive can contain full pre-normalized
telemetry. A future policy/redaction layer must protect sensitive fields before
TelemetryForge is used across trust boundaries.

**Do not expose v0.6.0 directly to an untrusted network or the public Internet.**
