# Live Multi-Cloud Chaos Runbook

Use this runbook only in isolated, non-production cloud projects/subscriptions with explicit cost and change approval. Do not target shared clusters, shared networks, production Kafka/PostgreSQL, or provider control-plane resources.

## Goal

Validate the same acceptance/accounting contract as `multicloudcheck` against real TelemetryForge edge nodes deployed in independent AWS, GCP, and Azure failure domains.

## Preconditions

- independent test clusters/networks in at least two cloud providers;
- cross-cloud edge replication configured with an explicit quorum;
- stable test Kafka/destination path;
- a fixed event corpus with unique IDs;
- proof/results storage outside the resources being faulted;
- an operator-defined rollback/heal command for every injected fault.

## Safe fault classes

Prefer faults that isolate **TelemetryForge test workloads**, not cloud-provider infrastructure:

1. scale one provider's edge deployment to zero;
2. deny test-edge traffic between one provider and the other test domains;
3. apply bounded network delay/loss to the test-edge path;
4. constrain a test WAL volume until readiness/backpressure engages;
5. stop two logical provider domains to verify quorum loss fails closed;
6. restore each fault and verify retained accepted events drain.

## Evidence to retain

For every run keep:

- exact Git commit and image digests;
- Terraform/Kubernetes configuration fingerprints;
- start/end timestamps;
- offered, accepted, rejected, uniquely delivered, retained, duplicate, lost, corrupted, and unaccounted counts;
- edge `/edge/status` snapshots before, during, and after the fault;
- raw fault-application/heal logs;
- a `.tfproof.json` artifact referencing the raw evidence.

A passing result requires every accepted event to remain accounted for. Rejections caused by unavailable durability quorum are expected and must be distinguishable from accepted telemetry.

## Claim boundary

A real-provider test supports only the topology, configuration, workload, and fault actually exercised. Do not generalize one successful run into a universal cloud-availability, WAN-latency, or zero-loss guarantee.
