# Security Policy

## v0.3.0 security posture

TelemetryForge is still a development/portfolio release. The gateway and worker
pipeline demonstrate distributed processing behavior, not a finished public
multi-tenant security boundary.

Not yet implemented:

- ingestion authentication
- tenant isolation and authorization
- rate limiting
- TLS termination in the application
- Kafka TLS/SASL configuration
- secrets management
- payload-level PII classification/redaction

**Do not expose v0.3.0 directly to an untrusted network or the public Internet.**

The worker consumes whatever records its Kafka credentials can access.
Production deployments must therefore apply least-privilege topic ACLs and
encrypted/authenticated broker connections.

Security controls will be introduced as the project reaches persistence,
policy-as-code, and production deployment milestones.
