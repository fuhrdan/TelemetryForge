# Changelog

All notable changes to TelemetryForge will be documented in this file.

## [0.1.0] - 2026-09-09

### Added
- Go HTTP ingestion gateway.
- `GET /health` and `GET /ready` service probes.
- `POST /api/v1/events` generic telemetry ingestion.
- `POST /api/v1/metrics` metric ingestion with numeric-value validation.
- Canonical versioned event envelope.
- Structured JSON logging.
- Graceful process shutdown.
- Request-size and strict JSON validation.
- Unit and HTTP handler tests.
- Docker image, Docker Compose, Makefile, and GitHub Actions foundation.
- Architecture, API, ADR, and local-development documentation.
