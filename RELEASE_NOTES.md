# TelemetryForge v2.6.0 Release Notes

**High-Performance Fast Path**

v2.6.0 accelerates the Durable Edge replay path while keeping every v2.1-v2.5 safety boundary intact. WAL fsync, configured replication quorum, mesh ownership/failover, cryptographic lineage, and ordered checkpointing remain authoritative; the new fast path begins only after those durability requirements have been satisfied.

## Highlights

### Bounded replay batching

The edge replay loop now reads up to a configurable batch of pending WAL records instead of forcing a one-record replay pass. The default is 64 records and the effective maximum is 1024.

`TELEMETRYFORGE_EDGE_REPLAY_BATCH_SIZE=64`

Batch publishers return one result per input item. TelemetryForge advances the WAL checkpoint only through the contiguous successful prefix. A failed item and everything after it remain pending and replayable.

### Kafka batch produce

The Kafka publisher now implements the optional batch contract using franz-go asynchronous produce callbacks. Events are submitted in input order, each callback records its own result, and the replay loop waits for the batch before advancing durable checkpoints.

JSON payloads use bounded reusable `bytes.Buffer` instances. A buffer remains owned by the Kafka record until its callback completes, then returns to the pool. Oversized buffers are discarded instead of being retained indefinitely.

### WAL scan allocation reduction

WAL frame scanning now uses a fixed-size frame-header array and a bounded reusable payload byte pool. JSON decoding reads directly from the payload bytes instead of allocating a temporary string representation.

### Lock-free bounded ring primitive

`internal/fastpath` adds a generic MPMC ring based on per-slot sequence counters. The ring rounds capacity to a power of two, never blocks, and refuses new writes when full rather than overwriting unread data. It is an acceleration primitive, never the only copy of accepted telemetry.

### Fast-path diagnostics

`GET /edge/status` now includes replay batch counters plus Kafka JSON buffer-pool reuse/allocation/discard counters.

### Reproducible performance evidence

`proof/fastpath-performance.sh --execute` runs correctness checks and five allocation-aware Go microbenchmark samples, preserves raw output, emits a parsed JSON benchmark report, and wraps the evidence in `.tfproof.json`.

The repository deliberately does **not** claim a universal event rate, p99.99 latency, or zero allocations across the complete data path. Microbenchmarks describe the runner that produced them. End-to-end capacity claims still require the existing full-stack k6 methodology with durability enabled and backlog/error behavior visible.

## Local validation boundary

The dependency-free fast-path, WAL, lineage, and replication packages were validated locally with race detection. The restricted build environment cannot download the repository's Go 1.27.1 toolchain or uncached franz-go/OpenTelemetry modules, so full edge/stream integration remains an authoritative CI check on Go 1.27.1.

See `docs/performance/fast-path-v2.6.md` and ADR 0057.
