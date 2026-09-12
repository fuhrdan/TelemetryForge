# TelemetryForge v2.2.0 Release Notes

**Release date:** 2026-09-12
**Milestone:** Replicated Durability

v2.2.0 moves the Durable Edge acceptance boundary from one fsynced disk to a configurable multi-node quorum. An origin edge still persists locally first, then non-local durability modes require durable peer acknowledgements across explicit failure domains before successful acceptance.

## Highlights

- Synchronous edge-to-edge replication of canonical WAL records.
- Receiver-side crash-safe replica logs with CRC32 framing and record validation.
- Idempotent exact retry for `(origin edge ID, edge sequence)`.
- Conflict rejection when the same origin sequence carries different content.
- Four durability modes: `local`, `regional`, `cross-region`, and `cross-cloud`.
- Quorum counts the local fsynced copy and is evaluated together with zone/region/cloud diversity.
- Regional durability requires a second availability zone in the origin region; acknowledgements from other regions do not inflate the regional quorum.
- Failed quorum attempts remain in the local WAL and are re-replicated before Kafka delivery.
- `/ready` checks live peer availability for non-local durability modes.
- `/edge/status` exposes replication mode, quorum, configured peers, and latest quorum result.
- Separate bearer-token authentication for internal replication endpoints.
- Bounded replica capacity plus durable release-through checkpoints and safe log compaction after downstream commit.
- Three-edge Docker Compose 2-of-3 regional durability topology.
- Kubernetes replica-storage/durability configuration hooks.
- Replication tests, proof harness, docs, and ADR 0053.

## Guarantee scope

A successful non-local v2.2 acceptance means the event was fsynced locally and the configured quorum/failure-domain rule was satisfied. The guarantee remains conditional on the stated durability assumptions; permanent destruction of every durable copy is outside the contract.

A request that returns failure after local fsync is indeterminate: the local WAL record remains recoverable and may later reach quorum and downstream delivery. Producers should retry with stable event IDs.

v2.2 does not claim consensus-based membership, automatic replica promotion, cryptographic lineage, formal proof of the protocol, or global sub-millisecond convergence.
