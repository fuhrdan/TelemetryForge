# Global Routing Mesh

TelemetryForge v2.3.0 adds a health-aware edge routing layer on top of the v2.1 WAL and v2.2 replicated-durability acceptance boundary.

The mesh does **not** decide whether an event is durable. Durability remains the responsibility of the origin WAL and replication quorum. The mesh decides where an already-accepted WAL record should make its downstream delivery attempt.

## Data path

```text
client
  |
  v
origin edge WAL -- fsync --> v2.2 durability quorum
  |
  v
mesh route selection
  |                 \
  | local            \ remote
  v                   v
local Kafka       peer /internal/v1/mesh/forward
                      |
                      v
                  peer local Kafka
```

The origin keeps its WAL record pending until the selected node has completed its downstream Kafka acknowledgement. If a remote attempt fails before success is observed, the origin retains the record and can retry through another eligible node. TelemetryForge therefore remains at-least-once; stable event IDs remain the deduplication key when an ambiguous network failure causes a retry.

## Topology advertisements

Each peer exposes an authenticated `GET /internal/v1/mesh/state` advertisement with:

- node ID;
- cloud, region, and zone;
- local downstream readiness;
- WAL pending-record count;
- WAL bytes and configured capacity;
- normalized WAL pressure;
- draining state; and
- observation timestamp.

Peers are actively probed. Stale, unhealthy, draining, or over-pressure nodes are excluded from new route decisions.

## Route policies

### `locality`

`locality` is the default. It selects the best currently available failure-domain tier first:

1. exact local zone;
2. same cloud and region;
3. same cloud;
4. another cloud/region.

Rendezvous hashing provides stable ownership inside the best available tier. This policy is intended to minimize unnecessary WAN and cross-cloud egress while preserving automatic failover.

### `global`

`global` puts all healthy nodes in one ownership set and uses rendezvous hashing across the set. It is intended for active-active deployments where spreading sources across regions/clouds is more important than strict locality.

For both policies, the route key is `tenant_id|source`, so related telemetry remains stable on the same owner while the topology is healthy.

## Deterministic ownership and failover

TelemetryForge uses highest-random-weight (rendezvous) hashing. A route decision is stable for the same key and eligible node set. When an owner becomes ineligible, the next-ranked candidate takes over without a centralized routing database.

Eligibility is fail-closed for new mesh routes when:

- downstream readiness is false;
- the node advertises `draining=true`;
- WAL pressure is at or above `TELEMETRYFORGE_MESH_MAX_PRESSURE`;
- the peer health probe fails; or
- the last peer advertisement is older than `TELEMETRYFORGE_MESH_STALE_AFTER`.

## Loop prevention

Remote forwarding uses `POST /internal/v1/mesh/forward`. The receiving handler validates the configured target node ID and publishes directly to its **local** Kafka publisher. It never invokes the mesh publisher again. A forwarded record therefore cannot recursively circulate between mesh nodes.

## Security

Mesh endpoints bypass public tenant API authentication because they are node-to-node control/data-plane endpoints. They have a separate bearer token boundary through `TELEMETRYFORGE_MESH_TOKEN`.

Use a secret manager in production and restrict the internal mesh endpoints with network policy, security groups, or equivalent controls. Do not expose them as public ingestion endpoints.

## Configuration

```text
TELEMETRYFORGE_MESH_POLICY=locality
TELEMETRYFORGE_MESH_TOKEN=...
TELEMETRYFORGE_MESH_PEERS=edge-b|https://edge-b.internal:8083|aws|us-west-2|b,edge-c|https://edge-c.internal:8083|gcp|us-central1|a
TELEMETRYFORGE_MESH_PROBE_INTERVAL=5s
TELEMETRYFORGE_MESH_TIMEOUT=2s
TELEMETRYFORGE_MESH_STALE_AFTER=15s
TELEMETRYFORGE_MESH_MAX_PRESSURE=0.90
TELEMETRYFORGE_MESH_DRAINING=false
```

`TELEMETRYFORGE_MESH_CLOUD`, `TELEMETRYFORGE_MESH_REGION`, and `TELEMETRYFORGE_MESH_ZONE` can override the edge replication failure-domain values when routing topology and durability topology intentionally differ.

## Operations

Authenticated public edge status includes a `mesh` snapshot:

```bash
curl -s http://localhost:8083/edge/status
```

A deterministic route can be inspected without sending an event:

```bash
curl -s 'http://localhost:8083/edge/route?key=tenant-a%7Ccheckout'
```

For planned maintenance, set `TELEMETRYFORGE_MESH_DRAINING=true` and restart/roll the node. Other nodes stop assigning new mesh-owned delivery to it after their next probe.

## Scope boundary

v2.3.0 intentionally uses static peer configuration plus active health exchange. Dynamic service discovery, route gossip, cryptographic topology signing, and autonomous route optimization remain future work. The release makes no claim that probe latency equals application delivery latency; observed probe RTT is exposed as operational evidence only.
