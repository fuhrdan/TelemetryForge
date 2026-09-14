# TelemetryForge v3.0.0 Release Notes

## Global Edge Fabric

TelemetryForge v3.0.0 consolidates the v2.1-v2.9 edge milestones into a single production-facing **Global Edge Fabric** contract. The release does not replace the Durable Edge, replication, routing, lineage, formal-verification, fast-path, autonomy, eBPF, or multi-cloud proof implementations. It makes their runtime composition explicit and easier to operate.

### Unified fabric status

Every edge now exposes:

```text
GET /edge/fabric/status
```

The response separates three readiness dimensions:

- `acceptance_ready` — new telemetry can safely cross the configured durable acceptance boundary;
- `delivery_ready` — a healthy local or remote mesh owner can currently deliver downstream;
- `audit_ready` — the cryptographic lineage identity is available.

The consolidated state is one of:

- `ready`: configured acceptance, delivery, and audit contracts are satisfied;
- `degraded`: durable acceptance remains safe, but a non-acceptance capability such as downstream delivery or optional eBPF collection is unavailable;
- `not_ready`: TelemetryForge cannot safely claim the configured acceptance contract.

This intentionally distinguishes a backlog-producing downstream outage from a loss of WAL/quorum safety.

### Fail-closed acceptance contract

The v3 fabric contract refuses to report safe acceptance when any required acceptance component is unavailable:

- local WAL is not writable or has reached its configured capacity;
- required replication quorum cannot be reached;
- cryptographic lineage identity is unavailable;
- eBPF was configured as required but is inactive.

A downstream route/Kafka outage does not automatically close acceptance if WAL + quorum remain healthy; the edge reports `degraded`, retains accepted work, and drains it after recovery.

### Release-contract checker

`telemetryforge-fabriccheck` is dependency-free and evaluates representative release states:

```bash
go test ./internal/fabric
go run ./cmd/fabriccheck
```

The checked scenarios are:

1. healthy cross-cloud fabric;
2. downstream partition with acceptance preserved;
3. replication-quorum loss;
4. WAL capacity exhaustion;
5. required eBPF collector unavailable.

`proof/global-edge-fabric.sh --execute` emits `.tfproof.json` evidence for the same contract.

### v3 compatibility

v3.0 is a consolidation release, not a destructive storage/API migration. It preserves:

- WAL v1/v2 read compatibility;
- public `/api/v1` ingestion/query surfaces;
- internal `/internal/v1` replication and mesh surfaces;
- `.tfincident` and `.tfproof.json` formats;
- at-least-once handling across ambiguous delivery failures;
- human-gated production lifecycle promotion;
- existing cryptographic lineage and multi-cloud proof boundaries.

No database migration is introduced solely for the new fabric status contract.

### Documentation and operations

v3.0 adds:

- ADR 0061: acceptance, delivery, and audit readiness are separate concepts;
- Global Edge Fabric architecture/operations documentation;
- explicit v3 upgrade and rollback guidance;
- CI validation of the fabric contract;
- a new manual Operational Proof scenario;
- aligned v3.0.0 versions across services, dashboard, and Kubernetes manifests.

## Claim boundary

The Global Edge Fabric status is a runtime composition/readiness contract. It does not claim exactly-once delivery, infallible disks, universal zero loss outside the configured durability assumptions, guaranteed public-cloud availability, or that a deterministic CI model reproduces a real provider outage. The narrower formal, cryptographic, performance, eBPF, and multi-cloud proof artifacts remain authoritative for those specific claims.
