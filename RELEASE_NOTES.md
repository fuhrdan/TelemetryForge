# TelemetryForge v2.1.0 Release Notes

**Release date:** 2026-09-12
**Milestone:** Durable Edge

v2.1.0 starts the TelemetryForge Global Edge Fabric roadmap by moving optional durable acceptance in front of Kafka. `telemetryforge-edge` fsyncs accepted telemetry to a local segmented WAL and replays it asynchronously into the existing Kafka pipeline.

## Highlights

- `telemetryforge-edge` executable using the existing ingestion/authentication surface.
- Segmented append-only WAL with fsync-before-acceptance semantics.
- Global edge and per-tenant/source monotonic sequences.
- CRC32 frame integrity and canonical-event SHA-256 verification.
- Atomic downstream checkpointing and ordered Kafka replay.
- Bounded retry during Kafka outage.
- WAL-capacity backpressure before overwrite.
- Closed delivered-segment compaction.
- Torn-tail crash recovery and fail-closed complete-frame corruption handling.
- `/edge/status` durable-state endpoint.
- Docker Compose persistent edge volume and Kubernetes StatefulSet/PVC.
- CI tests plus `proof/wal-crash-recovery.sh` for versioned `.tfproof.json` evidence.

## Guarantee scope

v2.1.0 provides a local **durable-after-acceptance** foundation. It does not claim cross-node quorum durability, cross-cloud survival, mathematically proven zero loss, or sub-millisecond global convergence. Those require later replication and formal-model milestones.
