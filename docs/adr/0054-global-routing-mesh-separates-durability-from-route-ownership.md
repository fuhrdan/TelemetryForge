# ADR 0054: Global routing mesh separates durability from route ownership

**Status:** Accepted for v2.3.0.

## Context

TelemetryForge v2.2.0 can prove that a non-local acceptance boundary has reached the configured durable failure domains, but all downstream replay still targets the origin node's Kafka dependency. A regional or cloud-local downstream outage can therefore leave safely accepted records queued even when another healthy TelemetryForge edge can reach a usable downstream path.

Routing and durability solve different problems. Combining them would make a routing optimization capable of silently weakening the acceptance contract.

## Decision

TelemetryForge adds a distinct global mesh layer after WAL persistence and replication quorum.

- The origin WAL and v2.2 replication manager remain the acceptance authority.
- The mesh owns downstream route selection only.
- Peer health is learned through authenticated active probes.
- New routes exclude unhealthy, stale, draining, and WAL-pressured nodes.
- Highest-random-weight hashing provides deterministic ownership inside the eligible locality tier or global set.
- Remote forwarding terminates at the selected peer's local Kafka publisher and cannot recursively invoke mesh routing.
- The origin commits and releases its durability replicas only after downstream delivery succeeds through the selected route.

## Consequences

A route outage does not erase an accepted event; the origin WAL keeps it pending. A topology change can move ownership without a central route database. Ambiguous remote network failures can still produce duplicate downstream delivery, so the system remains explicitly at-least-once and preserves stable event IDs for deduplication.

Static peer configuration is retained for v2.3.0. Dynamic discovery and cryptographic topology attestations are deferred so the first mesh release has a small, testable trust model.
