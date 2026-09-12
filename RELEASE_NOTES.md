# TelemetryForge v2.3.0 Release Notes

**Release:** Global Routing Mesh
**Date:** 2026-09-12

v2.3.0 turns the replicated Durable Edge into a health-aware routing fabric. The v2.2 WAL/quorum contract still controls successful acceptance; v2.3 adds a separate downstream ownership layer that can move replay between healthy edge/relay nodes without weakening durability.

## Highlights

- authenticated edge/relay topology advertisements;
- active health probing with stale-peer rejection;
- cloud, region, and zone failure-domain awareness;
- deterministic highest-random-weight (rendezvous) ownership;
- `locality` and active-active `global` routing policies;
- WAL-pressure and planned-drain route exclusion;
- automatic ordered-candidate failover;
- terminal peer forwarding with target identity validation and loop prevention;
- mesh topology in `/edge/status`;
- route inspection through `/edge/route?key=...`;
- three-edge active-active Docker Compose example;
- Kubernetes mesh configuration and secret surface;
- ADR 0054 and `proof/mesh-failover.sh`.

## Delivery semantics

The lifecycle is now:

```text
client
  -> local WAL fsync
  -> configured replication quorum
  -> successful acceptance
  -> deterministic mesh owner selection
  -> local Kafka OR authenticated terminal peer forward
  -> downstream acknowledgement
  -> origin WAL checkpoint
  -> peer replica release/compaction
```

A routing outage therefore does not erase an accepted event. The origin WAL remains pending until one route succeeds. The mesh remains explicitly at-least-once: a remote request that succeeds downstream but loses its HTTP response can be retried, so stable event IDs remain the deduplication boundary.

## Route policies

`locality` prefers the closest healthy failure-domain tier and only fails outward as required. `global` places every eligible configured node in one rendezvous ownership set for active-active distribution.

Nodes are excluded from new ownership when their downstream is not ready, their advertisement is stale, they are marked draining, active probing fails, or WAL pressure reaches the configured threshold.

## Scope boundary

v2.3 intentionally keeps static peer configuration. Dynamic cloud discovery, gossip, signed topology advertisements, WAN prediction, and autonomous cost/latency optimization remain future milestones.
