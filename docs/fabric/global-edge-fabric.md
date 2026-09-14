# Global Edge Fabric

TelemetryForge v3.0.0 consolidates the v2.1-v2.9 edge work into one explicit runtime contract. The fabric does not replace the underlying components; it reports whether their guarantees currently compose safely.

## Runtime contract

The edge exposes:

```text
GET /edge/fabric/status
```

The response has three independent dimensions:

- `acceptance_ready`: new telemetry can cross the configured durable acceptance boundary;
- `delivery_ready`: a healthy mesh owner can currently deliver downstream;
- `audit_ready`: the cryptographic lineage identity is available.

It also reports `ready`, `degraded`, or `not_ready` and a capability list for durable acceptance, replicated durability, global routing, cryptographic lineage, fast-path replay, and eBPF collection.

### Why acceptance and delivery are separate

A Kafka/downstream partition can leave the fabric in this state:

```text
state              = degraded
acceptance_ready   = true
delivery_ready     = false
audit_ready        = true
```

New accepted telemetry remains WAL-backed and, for non-local modes, quorum-replicated. The backlog drains when delivery recovers.

A quorum or capacity failure instead produces:

```text
state              = not_ready
acceptance_ready   = false
```

TelemetryForge must not claim successful acceptance when it cannot meet the configured durability contract.

## Capability composition

```text
client / OTel source
        |
        v
Durable Edge WAL + fsync
        |
        +--> cryptographic record lineage
        |
        v
failure-domain replication quorum
        |
        v
successful acceptance
        |
        v
Global Routing Mesh
        |
        +--> local or remote terminal owner
        |
        v
bounded fast-path replay / Kafka
        |
        v
downstream observability systems
```

Optional Linux eBPF counters enter at the same durable edge boundary. Autonomous worker controls remain downstream policy/shaping behavior and do not redefine the edge acceptance contract.

## Contract checker

The dependency-free checker exercises representative release states:

```bash
go test ./internal/fabric
go run ./cmd/fabriccheck
```

The checked scenarios are:

1. healthy cross-cloud fabric;
2. total downstream delivery partition with durable acceptance preserved;
3. required replication-quorum loss;
4. WAL capacity exhaustion;
5. required eBPF collection unavailable.

`proof/global-edge-fabric.sh --execute` captures these assertions in a `.tfproof.json` artifact.

## Claim boundary

The fabric contract is a readiness/composition model. It does not create exactly-once delivery, make disks infallible, guarantee public-cloud availability, or convert deterministic CI models into evidence of a real provider outage. Those claims remain bounded by the dedicated durability, formal, lineage, performance, and multi-cloud proof artifacts.
