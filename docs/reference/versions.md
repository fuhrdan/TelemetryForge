# Technology Version Baseline

Verified for the v2.8.0 release work on **2026-09-14**.

| Component | Project baseline | Rationale |
|---|---:|---|
| [Go](https://go.dev/doc/devel/release) | 1.27.1 | Current supported Go patch baseline |
| [TLA+ tools / TLC](https://github.com/tlaplus/tlaplus/releases) | 1.8.0 + repository SHA-256 pin | Formal safety model checker; upstream rolling asset is digest-verified before use |
| [Kubernetes](https://kubernetes.io/releases/) | 1.37.0 upstream / 1.36 EKS | Upstream stable plus current Amazon EKS standard-support boundary |
| [Terraform](https://github.com/hashicorp/terraform/releases) | 1.16.2 | Current Terraform project baseline |
| [AWS Provider](https://registry.terraform.io/providers/hashicorp/aws/latest) | 6.62.0 | Current project AWS provider baseline |
| [Apache Kafka](https://kafka.apache.org/community/downloads/) | 4.3.1 | Current project Kafka baseline |
| [franz-go](https://pkg.go.dev/github.com/twmb/franz-go) | 1.21.6 | Kafka producer/consumer baseline |
| [franz-go kadm](https://pkg.go.dev/github.com/twmb/franz-go/pkg/kadm) | 1.18.0 | Broker-derived consumer-group lag queries |
| [pgx](https://github.com/jackc/pgx/blob/master/CHANGELOG.md) | 5.10.0 | PostgreSQL client baseline |
| [TimescaleDB](https://github.com/timescale/timescaledb/releases) | 2.30.0 / PostgreSQL 17 | Durable telemetry/time-series store |
| [OpenTelemetry Go](https://pkg.go.dev/go.opentelemetry.io/otel) | 1.46.0 | Current trace API/SDK baseline |
| [OpenTelemetry Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/) | 1.44.0 | Reference version for the focused v1.1 semantic-attribute compatibility catalog |
| [Prometheus Go client](https://pkg.go.dev/github.com/prometheus/client_golang) | 1.24.1 | Process/application metrics |
| [OpenTelemetry Collector](https://opentelemetry.io/docs/collector/) | 0.160.0 | Local OTLP receive/batch/export |
| [Prometheus](https://prometheus.io/) | 3.13.3 | Local metrics store/query |
| [Grafana](https://grafana.com/grafana/) | 13.2.1 | Current stable local visualization baseline |
| [Tempo](https://grafana.com/oss/tempo/) | 3.0.2 | Local trace store/query |
| [k6](https://grafana.com/oss/k6/) | 2.2.0 | Reproducible load-test harness |
| [Node.js](https://nodejs.org/en/download) | 24.21.0 LTS | Dashboard development/CI runtime |
| [Next.js](https://nextjs.org/docs) | 16.3.4 | Dashboard framework |
| [React / React DOM](https://www.npmjs.com/package/react) | 19.2.8 | Dashboard UI runtime |
| [TypeScript](https://www.npmjs.com/package/typescript) | 5.9.3 | Compatibility pin for the selected stable Next.js build path |
| [Alpine Linux](https://www.alpinelinux.org/) | 3.24.1 | Backend runtime image |
| GitHub Actions | checkout/setup-go/setup-node v7; setup-java v6; setup-kubectl v5; setup-terraform v4 | CI action baseline |

## Dashboard container note

Node.js 24.21.0 LTS is the direct-development/CI target.

The dashboard runtime Dockerfile remains pinned to the latest exact Node 24
Alpine image that was previously verified for the repository rather than
floating on `node:24-alpine`.

## TypeScript compatibility pin

TypeScript 7 is a materially different native compiler distribution. The
selected stable Next.js build path still expects the legacy JavaScript compiler
API, so the dashboard remains intentionally pinned to TypeScript 5.9.3.

This is a compatibility decision, not a claim that 5.9.3 is the newest
TypeScript release.

## Dependency policy

Runtime/container versions are pinned instead of using `latest`.

Upgrades should be reviewed through CI with:

- unit/race/vet/build checks
- Kafka and TimescaleDB integration tests
- dashboard type/build checks
- observability configuration validation
- Kubernetes/Terraform validation
- k6 scenario inspection
- container builds

The dashboard still needs a registry-generated `package-lock.json` from a
successful npm-backed install so transitive npm dependencies are reproducible.

The Go module file now includes v0.9 observability dependencies. A real
registry-backed `go mod download`/test run in CI is authoritative for module
checksums that cannot be fetched inside restricted build sandboxes.
