# v2.6 High-Performance Fast Path

TelemetryForge v2.6 improves the path between the durable edge WAL and downstream delivery without moving the durability boundary.

## What changes

The fast path adds:

- bounded replay batches (64 records by default, capped at 1024);
- asynchronous Kafka batch submission so one replay batch does not require one broker round trip per record;
- bounded pooled JSON buffers whose lifetime extends through the Kafka callback;
- pooled WAL scan buffers and fixed-size frame headers;
- a bounded lock-free MPMC ring primitive for future high-rate in-process handoff paths;
- process-local counters for batch usage and JSON buffer reuse;
- repeatable Go microbenchmarks with `ns/op`, `B/op`, and `allocs/op` evidence.

## What does not change

The fast path never bypasses:

1. WAL append and fsync;
2. configured replicated-durability quorum;
3. deterministic mesh routing and failover;
4. ordered WAL checkpoint advancement;
5. cryptographic lineage and segment sealing.

A batch may partially fail. TelemetryForge commits only the contiguous successful prefix. The first failed record and everything after it remain replayable from the WAL. Duplicate downstream delivery remains possible under ambiguous failures, consistent with the documented at-least-once contract.

## Replay batching

`TELEMETRYFORGE_EDGE_REPLAY_BATCH_SIZE` controls the number of pending WAL records considered per replay pass.

```text
Default: 64
Minimum effective value: 1
Maximum effective value: 1024
```

A larger value can improve broker batching at the cost of more work per replay iteration. It does not change the maximum accepted WAL size or the durability acknowledgement policy.

## Pooled encoding

Kafka payload buffers are borrowed from a bounded `bytes.Buffer` pool. A buffer remains owned by its Kafka record until the delivery callback fires and is then returned to the pool. Buffers above the configured retention ceiling are discarded rather than retained indefinitely.

`GET /edge/status` exposes:

- `fast_path.replay_batch_size`
- `fast_path.batch_calls`
- `fast_path.batch_events`
- `fast_path.single_events`
- `fast_path.partial_failures`
- `kafka_buffer_pool.gets`
- `kafka_buffer_pool.reuses`
- `kafka_buffer_pool.allocations`
- `kafka_buffer_pool.discards`

These counters are operational diagnostics, not SLA measurements.

## Benchmarking

Run:

```bash
make proof-fastpath-performance
```

or directly:

```bash
proof/fastpath-performance.sh --execute
```

The harness captures five Go microbenchmark samples for the bounded ring, byte pool, and JSON buffer reuse path, preserves the raw output, produces a machine-readable benchmark report, and wraps both in a `.tfproof.json` artifact.

Do not turn one runner's microbenchmark into an end-to-end events-per-second claim. Publish end-to-end capacity only with the existing k6 operational-proof methodology, full durability enabled, environment details recorded, and broker/backlog behavior visible.
