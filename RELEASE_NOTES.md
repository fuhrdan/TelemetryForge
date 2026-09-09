# TelemetryForge v0.8.0 Release Notes

## Kubernetes, Cardinality Firewall, Terraform, Policy-as-Code & Shadow Pipeline

v0.8.0 is a cumulative release. The reliability/repository hardening work done
on `v0.7.0-dev`, plus the planned Kubernetes/Cardinality Firewall milestone, is
included here; there is intentionally no `v0.7.0` release tag.

### Cardinality Firewall

- bounded 64-register HyperLogLog cardinality estimator
- per source / event type / dimension tracking
- SHA-256-derived value fingerprints instead of raw identifier values
- minimum observation window before generic one-hour projection
- early warnings for identifier-shaped keys
- bounded tracked-dimension state
- repeated finding/diff rate limiting
- `allow`, `drop_tag`, and `quarantine` actions
- idempotent quarantine evidence keyed by event ID
- dashboard/API findings view
- deterministic unit tests

### Policy-as-Code

- strict versioned JSON policy documents
- active policy file
- candidate shadow policy file
- glob matching for source/type/dimension
- startup validation
- `telemetryctl policy validate`
- Kubernetes policy ConfigMap
- Docker image policy packaging

### Shadow Pipeline

- active and candidate policy evaluate the same normalized event
- candidate never mutates active behavior
- active/shadow action disagreements persisted
- dashboard shadow-difference view
- shadow mode can be disabled explicitly

### Kubernetes

- Kustomize base
- gateway Deployment + Service
- worker Deployment with dedicated admin health listener
- dashboard Deployment + Service
- dependency-aware readiness probes
- liveness probes
- resource requests/limits
- HPA examples
- PodDisruptionBudgets
- policy ConfigMap
- Secret template
- example Ingress
- worker scale ceiling documented against Kafka partition count

### Terraform

- Terraform 1.16.1 baseline
- AWS provider 6.62.0
- VPC
- public/private subnets
- Internet Gateway
- NAT Gateway
- EKS cluster
- managed EKS node group
- IAM roles/policy attachments
- EKS 1.36 default, matching current EKS standard support

### Reliability carried forward from v0.7.0-dev

- contiguous same-partition Kafka acknowledgement coordination
- rebalance-safe bounded poll batches
- dependency-aware gateway readiness
- SSE write-timeout hardening
- bounded automatic-incident detector state
- Flight Recorder retry deduplication in frozen incident timelines
- ingestion-time and dedup-maintenance database indexes
- expanded GitHub/repository documentation and hygiene checks

### Dashboard

The existing operations dashboard now adds:

- Cardinality Firewall findings
- observed/projected unique counts
- active versus shadow indicator
- action display
- active-to-shadow policy decision differences

### Demo

```bash
docker compose up --build
make demo-cardinality
```

Then open `http://localhost:3000`.

### Known limitations

- Cardinality estimates are currently replica-local. Multiple worker replicas
  can under-estimate cluster-wide unique cardinality.
- `quarantine` establishes a routing contract, but no external observability
  vendor output connector exists yet.
- findings, shadow diffs, and quarantine evidence do not yet have automated pruning.
- Kubernetes HPA uses resource metrics, not Kafka lag.
- the Terraform module provisions EKS/networking, not Kafka/TimescaleDB.
- the checked-in Kubernetes Secret is an example only.
- authentication/tenant isolation and Kafka TLS/SASL remain future hardening.
- full Next.js/package-lock regeneration still requires registry-backed npm
  access.

These limitations are documented deliberately rather than hidden behind a
"production ready" claim.
