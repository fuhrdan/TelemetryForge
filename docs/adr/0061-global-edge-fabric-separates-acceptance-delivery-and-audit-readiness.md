# ADR 0061: Global Edge Fabric separates acceptance, delivery, and audit readiness

## Status

Accepted for v3.0.0.

## Context

TelemetryForge accumulated distinct correctness boundaries across v2.x: local WAL durability, replicated quorum, global route ownership, cryptographic lineage, fast-path replay, optional eBPF collection, autonomous shaping, and multi-cloud proof. A single boolean health signal would hide important failure modes.

In particular, a downstream/Kafka or mesh delivery outage does not necessarily make the edge unsafe to accept telemetry: the WAL and requested replication quorum may still be healthy and able to retain accepted work. Conversely, a lost durability quorum or exhausted WAL must not be presented as a merely degraded downstream condition.

## Decision

v3.0 introduces a Global Edge Fabric contract with three explicit readiness dimensions:

- **acceptance readiness**: the local WAL can persist additional data, required replication quorum is reachable, the lineage signer is available, and any required eBPF collector is active;
- **delivery readiness**: the routing mesh has a healthy terminal downstream owner;
- **audit readiness**: cryptographic lineage identity is available.

The consolidated fabric state is:

- `ready` when acceptance, delivery, and audit requirements are satisfied;
- `degraded` when durable acceptance remains safe but a non-acceptance capability such as downstream delivery or optional eBPF collection is unavailable;
- `not_ready` when TelemetryForge cannot safely claim its configured acceptance contract.

`GET /edge/fabric/status` exposes the dimensions and capability records. It does not weaken the existing WAL, replication, mesh, or lineage implementations; it evaluates their live state.

## Consequences

Operators can distinguish a backlog-producing dependency outage from a loss of the safe acceptance boundary. Alerting and load balancers can make policy decisions from explicit dimensions instead of one ambiguous health bit.

The contract is intentionally not an exactly-once guarantee, a WAN availability guarantee, or proof that every external dependency is healthy.
