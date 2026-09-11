# TelemetryForge v1.9.0 Development Release Notes

## Scale, HA & Operational Proof

v1.9.0 is intentionally less about adding another vendor integration and more
about proving the control plane under load, failure, maintenance, and recovery.

The central rule is:

> A test threshold is not an achieved performance claim. A passing claim must
> come from an executed, versioned proof artifact.

## `.tfproof.json`

Added the versioned TelemetryForge Operational Proof format.

Each result records:

- scenario and pass/fail/error status;
- exact Git commit;
- execution timestamps;
- OS/architecture/CPU/memory/tool context;
- SHA-256 of relevant configuration;
- observed measurements with units/sources;
- explicit assertions with expected/observed values;
- SHA-256/size fingerprints of raw evidence.

A `pass` artifact is rejected if any assertion failed or if there is no
measurement/evidence.

## Proof CLI / API / dashboard

Added:

```bash
telemetryctl proof verify --file proof/results/run.tfproof.json
telemetryctl proof record --file proof/results/run.tfproof.json
telemetryctl proof list --limit 25
```

Migration `016_operational_proof.sql` stores verified proof artifacts plus the
whole-file SHA-256/size.

Re-recording the same run/hash is idempotent. Reusing a run ID with different
bytes is rejected.

Added:

```text
GET /api/v1/proofs
```

and an Operational Proof dashboard panel.

## Reproducible k6 capture

`scripts/run-benchmark.py` runs the existing smoke/sustained/backpressure k6
scenarios with summary export and records measurements that actually exist in
the k6 result.

It does not synthesize missing throughput, p95, failure-rate, or dropped-event
numbers.

## Local failure proof

`scripts/run-proof.py` adds executable:

```text
preflight
kafka-outage
database-outage
connector-outage
worker-failover
```

Kafka outage proof verifies the gateway does not return false `202 Accepted`
while Kafka is paused, then measures observed recovery after Kafka resumes.

Database outage proof keeps Kafka available, pauses TimescaleDB, verifies a
protected event can still be durably accepted into Kafka, then verifies it
reaches primary persistence after the database resumes.

Connector outage proof fans one event to healthy Kafka plus an intentionally
unreachable HTTP destination and passes only if the healthy lane delivers while
the failed lane independently retries or dead-letters.

Worker failover scales to two consumers, terminates one, submits protected audit
telemetry, and verifies every gateway-accepted proof event reaches TimescaleDB.

Disruptive scenarios require explicit `--execute`.

## Backup / restore proof

`scripts/proof-db-restore.py --execute` performs a real custom-format
TimescaleDB/PostgreSQL dump, restores it into a temporary database, uses
TimescaleDB pre/post restore handling, and compares selected application table
row counts.

The raw dump is retained under ignored `proof/results/raw/` and fingerprinted from the
summary artifact.

No RPO/RTO number is claimed unless the executed artifact records one.

## Kubernetes rolling proof

Added `scripts/proof-k8s-rollout.py`.

Because it restarts workloads, execution requires both:

```text
--execute
--acknowledge-cluster-change
```

The harness captures PDB/deployment state before and after each rollout and
measures the observed rollout duration.

## Kubernetes HA hardening

Gateway/router/dashboard now use:

```text
RollingUpdate maxUnavailable: 0
maxSurge: 1
minReadySeconds: 5
progressDeadlineSeconds: 600
```

Worker uses:

```text
maxUnavailable: 1
maxSurge: 1
minReadySeconds: 5
progressDeadlineSeconds: 600
```

All four workloads use hostname topology spreading.

Router gains the previously missing:

- PodDisruptionBudget;
- HorizontalPodAutoscaler (`2..6`).

Compose services also receive explicit graceful-stop periods.

## Manual GitHub Operational Proof workflow

Added `.github/workflows/operational-proof.yml` with `workflow_dispatch` for:

- k6 smoke/sustained/backpressure;
- Compose preflight;
- Kafka outage;
- PostgreSQL outage;
- connector outage isolation;
- worker failover;
- database backup/restore.

The workflow verifies generated `.tfproof.json` and uploads raw evidence/result
files as CI artifacts.

It is manual rather than quietly creating benchmark claims on every PR.

## Documentation

Added human-readable guides for:

- operational proof artifacts;
- high availability;
- chaos/failure testing;
- disaster recovery;
- rolling upgrade proof;
- v1.9 delivered scope;
- ADRs 0046-0048.

## No fabricated benchmark results

This development release intentionally includes **no made-up showcase
throughput or recovery numbers**.

Run the proof harness on representative hardware/infra and publish the resulting
artifact if a measured claim is needed.

## Known limitations

- Local Compose worker-failover tests process crash/restart plus consumer-group
  continuity; they are not a multi-node machine-loss proof.
- Kubernetes rolling proof verifies rollout and ready replica convergence; add
  concurrent traffic if a specific zero-request-loss claim is required.
- Backup/restore proof compares selected application table row counts and does
  not prove every higher-level semantic relationship.
- Multi-region active/active failover is not claimed by v1.9.
- Full achieved performance remains environment-specific.
