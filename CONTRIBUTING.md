# Contributing to TelemetryForge

Contributions should keep the project runnable, documented, and easy to reason
about. A feature is not complete when only the code exists.

## Prerequisites

For direct development:

- Go 1.27.1+
- Node.js 24.21 LTS for the dashboard
- Docker with Docker Compose for Kafka/TimescaleDB integration
- Python 3 for demo/documentation helper scripts

The easiest full environment remains:

```bash
docker compose up --build
```

## Development workflow

1. Branch from `main` or the current development branch.
2. Keep one coherent change per branch where practical.
3. Add tests with behavior changes.
4. Update human-readable documentation when behavior, configuration, failure
   semantics, deployment, or architecture changes.
5. Add an ADR for a significant architectural decision.
6. Run the relevant checks before opening a pull request.

Backend checks:

```bash
make check
```

Documentation/repository checks:

```bash
make docs-check
```

Dashboard:

```bash
make dashboard-build
```

Integration tests with Kafka/TimescaleDB running:

```bash
make integration-test
```

## Code standards

### Go

- Keep HTTP, Kafka, worker, reliability, and storage concerns separated by
  package boundaries.
- Document exported identifiers with GoDoc comments.
- Explain **why** around concurrency, acknowledgement ordering, backpressure,
  retry, idempotency, and security-sensitive logic.
- Prefer bounded queues/concurrency to implicit unbounded goroutine growth.
- Propagate `context.Context` through blocking/external operations.
- Do not claim exactly-once behavior unless the entire end-to-end contract can
  actually prove it.

### TypeScript / dashboard

- Keep API types explicit.
- Prefer small native/browser capabilities before adding dependencies that do
  not solve a concrete requirement.
- Preserve accessibility basics: semantic controls, labels, keyboard-usable
  interactions, and readable status text independent of color.

### SQL / migrations

- Migrations must be idempotent where the current migration runner expects
  repeat execution.
- Explain retention/index choices in `docs/storage/`.
- Do not silently change a delivery/idempotency guarantee in SQL alone.

### Policy changes

Treat policy files as code:

```bash
go run ./cmd/telemetryctl policy validate --file policies/active.json
go run ./cmd/telemetryctl policy validate --file policies/shadow.json
```

For a potentially destructive policy change, prefer changing the shadow policy
first and reviewing stored differences before promoting it to active.

### Kubernetes / Terraform

Render Kubernetes changes:

```bash
make k8s-render
```

Validate Terraform changes:

```bash
make terraform-check
```

Do not commit real Kubernetes secrets, Terraform state, or cloud credentials.

### Replay / cost changes

Replay safety is a release invariant.

A change that can publish replay output must keep the library-level topic
restriction to `telemetry.replay*`; a CLI-only safety check is insufficient.

Cost-simulation changes must document assumptions and must not add default
dollar pricing.

### Observability changes

Before adding a Prometheus label, verify it is bounded.

Do not use:

```text
event ID
incident ID
correlation ID
raw URL
arbitrary source
arbitrary telemetry tag/value
error message
```

as metric labels.

Trace attributes have a different cardinality model but still require privacy
review.

Validate observability config with CI or:

```bash
make observability-check
```

### Load-test changes

Keep k6 scenarios deterministic/reviewable and pinned.

A benchmark result needs the environment metadata described in
`docs/performance/benchmark-methodology.md`.

Do not add unsupported throughput claims to README/release notes.

### Schema Intelligence changes

Schema drift must remain advisory unless a future version introduces an
explicit versioned enforcement policy.

Do not:

- put raw telemetry values into schema fingerprints;
- make schema state cross tenant boundaries;
- treat one missing optional field as breaking;
- expand payload inspection without preserving explicit bounds; or
- make an advisory registry outage block telemetry by default.

A semantic-convention catalog update should cite the upstream OpenTelemetry
version it was reviewed against.

### Telemetry Router invariants

Routing changes must preserve these release guarantees:

- external destination I/O never runs inside the primary Kafka processing worker;
- one destination's retry/DLQ state cannot block an unrelated destination;
- shadow routing records candidate decisions but performs no candidate I/O;
- routing outbox writes remain idempotent by tenant/event/destination;
- destination credentials come from environment/secret-manager inputs rather than committed routing JSON.

### Portable incident archive changes

`.tfincident` is a security/integrity boundary.

Changes must preserve:

- strict format-version handling;
- member SHA-256 verification;
- ZIP path/member/size bounds;
- no implicit production-pipeline replay on import;
- no automatic activation of archived configuration;
- explicit tenant-remap acknowledgement;
- authenticated encryption when encryption is requested.

Do not add password encryption without a reviewed, versioned KDF design.

## Documentation standards

Human-readable documentation is a project requirement.

Update documentation when a change affects:

- public APIs or event format
- environment variables
- retry/DLQ behavior
- retention/idempotency
- incident capture
- scaling/deployment
- dashboard behavior
- security boundaries

Run:

```bash
python3 scripts/check-docs.py
```

The checker validates required project documents, relative links, ADR numbering,
and accidental local artifact paths.

## ADRs

Add an ADR under `docs/adr/` when a change has a meaningful long-term trade-off.
Use the next sequential four-digit number and include:

- Status
- Date
- Context
- Decision
- Alternatives considered
- Consequences

Do not rewrite an accepted ADR to pretend the original decision never existed.
Add a superseding ADR when the architecture changes materially.

## Commit style

Use conventional commits where practical:

```text
feat: add cardinality estimator
fix: preserve source offset when DLQ publish fails
test: cover automatic incident cooldown
docs: document kubernetes scaling limits
chore: refresh supported toolchain versions
```

## Pull-request checklist

Before requesting review:

- [ ] behavior has tests
- [ ] `make check` passes
- [ ] `make docs-check` passes
- [ ] dashboard changes build/type-check
- [ ] external-integration changes have integration coverage
- [ ] documentation and configuration reference are current
- [ ] ADR added/updated when architecture changed
- [ ] no credentials, real customer telemetry, or local artifact paths included
- [ ] `CHANGELOG.md` updated for user-visible/operational changes

See [the testing guide](docs/development/testing.md) and
[release process](docs/development/release-process.md).

### Connector Platform changes

Connector code is a protocol boundary, not a second delivery state machine. Keep retry/DLQ/fallback ownership in the Telemetry Router, use environment-backed secrets, bound connector metadata/response handling, keep connector health separate from router readiness, and preserve legacy Kafka/HTTP routing compatibility.
