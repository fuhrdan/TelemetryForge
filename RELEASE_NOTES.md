# TelemetryForge v2.9.0 Release Notes

## Multi-Cloud Proof

TelemetryForge v2.9.0 turns the v2.2-v2.8 cross-cloud durability architecture into a reproducible failure-accounting milestone. The release adds cloud foundations for AWS, GCP, and Azure, a deterministic multi-cloud fault checker, a destructive logical-cloud Compose suite, and `.tfproof.json` evidence for accepted-event accounting.

### Deterministic failure-domain checker

`telemetryforge-multicloudcheck` models AWS `us-west-2`, GCP `us-central1`, Azure `westus2`, and a bare-metal Denver edge. The default model requires durable copies in at least two distinct clouds and targets three copies while capacity exists.

The checker runs six schedules:

- AWS cloud outage after durable acceptance;
- GCP regional partition while ingestion continues;
- disk/capacity pressure that removes durability quorum;
- packet-loss/latency ambiguity that causes duplicate delivery attempts;
- cascading AWS + Azure outage after three-copy persistence.
- total delivery-path partition with accepted telemetry retained until recovery.

Every offered event is classified. A passing run requires zero lost, zero corrupted, and zero unaccounted accepted events, and all accepted events must be uniquely delivered after recovery. When durability quorum cannot be met, new telemetry is rejected before successful acknowledgement.

### Destructive logical-cloud suite

`proof/multi-cloud-compose.sh --execute` runs an opt-in destructive integration exercise against a local Compose topology labeled as AWS/GCP/Azure. It verifies baseline cross-cloud acceptance, continued acceptance after one logical cloud is stopped, fail-closed behavior after both remote clouds are stopped, and restored acceptance after recovery.

The containers still share one machine. This is runtime failover evidence, not a claim that a real public-cloud regional outage occurred.

### Cloud deployment foundations

The repository now includes Kubernetes/network Terraform foundations for:

- AWS EKS (`infra/terraform/aws`);
- GCP GKE (`infra/terraform/gcp`);
- Azure AKS (`infra/terraform/azure`).

Kafka and PostgreSQL/TimescaleDB remain external dependencies so each organization can use its approved managed or self-hosted platforms.

### Operational evidence

`proof/multi-cloud-proof.sh --execute` emits a `.tfproof.json` artifact plus the raw machine-readable accounting report. CI race-tests the dependency-free checker and validates all three Terraform foundations.

### Explicit claim boundary

v2.9 proves named protocol/accounting properties under the checked model and supplies a destructive logical-cloud integration suite. It does not claim universal zero loss, public-cloud availability, WAN latency, or real-provider outage survival unless those properties are separately measured in an isolated real-provider run with corresponding evidence.
