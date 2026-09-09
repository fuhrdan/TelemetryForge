# Security Policy

TelemetryForge is under active development.

## v0.2.0 security posture

This release does **not** yet provide:

- client authentication
- API authorization
- tenant isolation
- API rate limiting
- Kafka SASL/TLS configuration
- secret-management integration
- payload-level PII redaction

The bundled Kafka Compose configuration uses plaintext listeners and is intended only for local development.

**Do not expose v0.2.0 directly to an untrusted network or public Internet.**

Security controls will be introduced incrementally and documented in the release where their behavior becomes part of the supported architecture.

## Reporting

Please report suspected vulnerabilities privately to the repository owner rather than opening a public issue containing exploit details or secrets.
