# ADR 0057: Fast Path Cannot Bypass Durability Boundaries

**Status:** Accepted for v2.6.0.

## Context

TelemetryForge needs to reduce replay overhead and broker round trips without invalidating the safety properties added in v2.1-v2.5. A conventional performance shortcut would acknowledge before fsync, reduce replication requirements, silently drop work from a full in-memory queue, or advance checkpoints past failed batch items.

Those optimizations would make benchmark numbers look better by weakening the system being measured.

## Decision

Performance optimizations are permitted only after the durable acceptance boundary. WAL fsync, configured replication quorum, cryptographic lineage, and ordered checkpoint semantics remain unchanged.

Replay may batch pending records. Batch publishers must return one result per input record. TelemetryForge advances the WAL checkpoint only through the contiguous successful prefix. A failed or incomplete batch result leaves the first unresolved record and all following records pending.

Reusable pools are bounded. The lock-free ring refuses new writes when full rather than overwriting unread values. The WAL remains the source of truth; in-memory acceleration structures are never the sole copy of accepted telemetry.

## Consequences

- performance work cannot manufacture throughput by disabling durability;
- partial batch failures may replay already-delivered later records, preserving at-least-once rather than pretending exactly-once delivery;
- benchmark artifacts must describe the environment and cannot be generalized into universal capacity claims;
- future eBPF or native fast paths must obey the same boundary.
