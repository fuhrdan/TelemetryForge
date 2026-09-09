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
