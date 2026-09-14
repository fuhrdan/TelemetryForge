# Multi-Cloud Proof

TelemetryForge v2.9.0 adds a reproducible failure-domain/accounting proof for the durable edge fabric.

## What is checked

`telemetryforge-multicloudcheck` models four independent failure domains:

- AWS `us-west-2`;
- GCP `us-central1`;
- Azure `westus2`;
- a bare-metal Denver edge.

The default acceptance contract requires two distinct durable clouds and targets three durable copies when capacity is available. Six named schedules are exercised:

1. AWS cloud outage after acceptance;
2. GCP regional partition while ingestion continues;
3. disk/capacity pressure that leaves fewer than two writable clouds;
4. packet-loss/latency ambiguity that forces duplicate delivery attempts;
5. cascading AWS + Azure outage after three-copy persistence.

For every scenario the report records offered, accepted, rejected-before-acceptance, uniquely delivered, retained, duplicate attempts, lost, corrupted, and unaccounted events. After recovery, every accepted event must be uniquely delivered.

Run:

```bash
go run ./cmd/multicloudcheck --events 10000 --output /tmp/multicloud.json
proof/multi-cloud-proof.sh --execute
```

## Local logical topology

Use the Compose override to exercise cloud-aware routing/replication labels on one machine:

```bash
docker compose -f docker-compose.yml -f deployments/multicloud/docker-compose.override.yml up -d --build
```

This is useful integration evidence, but all containers still share one host. It is **not** evidence that AWS, GCP, and Azure independently survived a real outage.

## Claims boundary

The deterministic proof validates TelemetryForge's event-accounting and fail-closed acceptance semantics under the named model. It does not prove WAN latency, public-cloud availability, provider control-plane behavior, or a universal zero-loss guarantee outside the configured durability assumptions.

For real-provider testing, use the [Live Multi-Cloud Chaos Runbook](live-chaos-runbook.md).
