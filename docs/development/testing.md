# Testing Strategy

TelemetryForge treats tests as part of each release, not a final hardening
phase.

## Local fast checks

```bash
make check
```

This runs formatting, `go vet`, unit tests, and Go builds.

Documentation/repository checks:

```bash
make docs-check
```

Dashboard build/type checking:

```bash
make dashboard-build
```

## Integration tests

With Kafka and TimescaleDB running locally:

```bash
make integration-test
```

The integration suite exercises real broker/database boundaries. Tests skip an
external dependency only when its corresponding environment variable is not
provided; CI provides both variables in dedicated jobs.

## What should be unit tested?

Unit tests are expected for:

- canonical envelope validation
- retry classification/backoff behavior
- worker acknowledgement/failure routing
- normalization and processor ordering
- automatic incident thresholds/cooldowns
- API parameter validation

## What should be integration tested?

Integration tests are expected for behavior that depends on real external
semantics, including:

- Kafka publish/consume acknowledgement
- idempotent TimescaleDB persistence
- database migrations
- Flight Recorder incident freezing

## v0.8.0 coverage

The release adds/targets tests for:

- cardinality estimation and threshold policy
- deterministic allow/drop/quarantine decisions
- bounded estimator/reporting state
- active versus shadow policy decisions
- strict policy validation
- worker health/readiness behavior
- Kubernetes manifest rendering
- Terraform format/init/validate
- quarantine/finding/diff persistence
- consumer scaling guidance against topic partition count

## Performance claims

Do not place throughput or latency numbers in the README unless they come from a
checked-in reproducible load-test methodology and results file. k6 benchmarking
is scheduled for the v0.9.0 performance milestone.


## v0.9.0 coverage

New unit/integration checks cover:

- replay analysis effects
- replay production-topic rejection
- explicit replay namespace publication
- event-time policy evaluation
- cost simulation byte/series effects
- explicit pricing behavior
- pricing currency validation
- replay/cost database history
- replay/cost history API routes
- Prometheus route-pattern cardinality
- invalid negative Kafka lag suppression

CI also validates:

- Prometheus configuration
- OpenTelemetry Collector configuration
- Tempo configuration
- provisioned Grafana JSON
- all three k6 scenarios with `k6 inspect`

## Benchmark tests are not CI throughput gates

The checked-in k6 scripts are methodology and regression tools.

CI inspects their syntax/options but does not run a full distributed performance
benchmark on arbitrary shared GitHub-hosted runners and publish the number as a
product claim.

A published benchmark must use the procedure in
`docs/performance/benchmark-methodology.md`.


## v1.0.0 security/evidence coverage

New checks include:

- API-key scope enforcement
- health probe auth exemption
- client `tenant_id` spoof rejection
- tenant-isolated Cardinality Firewall state
- same event ID independently accepted by two tenants
- API redaction without source-event mutation
- Kafka TLS/SASL configuration validation
- Evidence Graph support/contradiction behavior
- explicit non-causality wording
- Evidence Graph API/storage boundary
- dashboard server-proxy TypeScript compilation
- production Kustomize overlay rendering in CI

The real Go 1.27.1 dependency-backed build remains the authoritative CI gate
when the local sandbox cannot fetch/currently run that toolchain.


## v1.1.0 Schema Intelligence coverage

New coverage includes:

- deterministic value-free schema fingerprints;
- legacy OpenTelemetry semantic attribute warnings;
- semantic payload type mismatches;
- version diff compatibility classification;
- sparse fields not becoming required prematurely;
- same-version type changes becoming breaking drift;
- `schema_url` changing under the same application version producing warning;
- fail-open default worker behavior;
- optional fail-closed transient classification;
- schema API validation; and
- dashboard schema/history TypeScript compilation.

Database integration coverage verifies idempotent observation counts and
version/drift history when the TimescaleDB/PostgreSQL CI service is available.


Additional Schema Intelligence hardening tests cover:

- accumulated registry field-cap enforcement;
- deduplicated field-limit warning behavior; and
- prune-cutoff validation before database mutation.


## v1.2.0 Distributed Cardinality coverage

New coverage includes:

- exact low-cardinality merging across two independent shared-tracker instances;
- tenant-scoped shared state lookup;
- hourly local tracker rollover for replay/cost semantic parity;
- series-budget validation and non-destructive behavior;
- persisted budget status transitions;
- distributed state API route;
- budget API route;
- dashboard Distributed Cardinality / Series Budgets TypeScript compile; and
- migration/config/documentation structural checks.

The TimescaleDB integration job is the authoritative test that the atomic
`set_byte(max(...))` HLL update works against real PostgreSQL/TimescaleDB.


## v1.3.0 Telemetry Router coverage

New unit/integration checks cover:

- tenant/source/type/severity/tag route matching
- fan-out destination deduplication
- unmatched fallback routing
- shadow routing with no candidate enqueue
- failure-fallback cycle rejection
- HTTP idempotency/tenant headers
- environment-backed generic secret headers and static-secret rejection
- non-2xx HTTP delivery failure
- bounded retry delay
- independent destination failure versus healthy delivery
- routing outbox idempotency
- independent destination claim/delivery
- per-destination dead-letter persistence
- atomic failure fallback enqueue
- routing API/CLI compile surfaces
- router Docker/Kubernetes configuration

The storage integration test uses the real PostgreSQL migration in CI.

## v1.4.0 shaping coverage

Tests cover protected errors/high latency, deterministic sampling, pressure
floors, transformations, payload dropping, shadow isolation, successful sampled-
out early stop, fail-open audit behavior, API validation, and frozen-incident
preview calculations.


### v1.4 retry-stability coverage

Adaptive-shaping tests explicitly cover:

- retry-stable first-decision reuse when queue pressure changes;
- exactly-once minute-stat aggregation through the decision ledger; and
- fail-open behavior when decision persistence is unavailable.


## v1.5.0 archive coverage

Unit checks cover:

- unencrypted ZIP round trip;
- AES-256-GCM encrypted round trip/failure behavior;
- wrong-key and ciphertext-tamper rejection;
- member SHA-256 mismatch rejection;
- unsafe ZIP member path rejection;
- tenant-consistency validation;
- HTML escaping of telemetry-controlled content.

TimescaleDB integration covers:

```text
freeze -> export -> verify -> target-tenant import -> provenance
```

and proves a second import of the same archive ID into one tenant fails.

CI/full environments should also run:

```bash
go test ./internal/incidentarchive
go test ./tests/integration
```


## v1.7.0 Change Intelligence coverage

Checks include:

- deployment/change marker normalization;
- insufficient-evidence guardrails;
- material error/latency regression classification;
- observed blast-radius calculation;
- change recorder only capturing operational changes;
- authenticated change ingestion canonicalization;
- change analysis API behavior;
- structured Evidence Graph change and rollback relationships;
- `.tfincident` change marker serialization/count verification;
- TimescaleDB before/after regression plus rollback integration scenario;
- dashboard TypeScript compilation.

## v1.8.0 Connector Platform coverage

Focused tests cover connector catalog/protocols, vendor credential health probes, permanent/transient failure classification, OTLP path selection, Prometheus label allowlisting, and legacy routing translation. CI remains responsible for the full dependency-backed suite.
