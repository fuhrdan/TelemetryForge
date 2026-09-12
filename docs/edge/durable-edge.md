# Durable Edge Ingestion

TelemetryForge v2.1.0 adds an optional `telemetryforge-edge` service that places a local crash-safe write-ahead log (WAL) in front of the existing Kafka ingestion path.

```text
Client -> Edge -> append + CRC/hash -> fsync -> HTTP acceptance
                   |
                   `-> asynchronous ordered replay -> Kafka
```

The guarantee is deliberately scoped: once the edge returns successful acceptance, the event remains recoverable from the configured WAL until downstream delivery is checkpointed, provided the WAL storage itself remains available and uncorrupted. TelemetryForge does not claim survival after permanent destruction of every durable copy.

Each record contains an edge ID, destination topic, globally monotonic `edge_sequence`, monotonic tenant/source `source_sequence`, acceptance timestamp, SHA-256 of the canonical event JSON, and the original event. The binary frame also carries CRC32. Starting in v2.4, WAL format v2 additionally links each record to the previous record hash and carries its own deterministic lineage hash.

On startup, a partial final frame is treated as an interrupted write and truncated to the last verified record. A completed frame with a CRC/hash failure fails closed. Delivery checkpoints are atomically replaced only after downstream acknowledgement, so a crash can create a safe duplicate replay but not intentionally skip an uncheckpointed record.

`TELEMETRYFORGE_EDGE_WAL_MAX_BYTES` is a hard local durability budget. When another record cannot fit, the edge rejects new acceptance instead of overwriting pending telemetry.

## Configuration

```bash
TELEMETRYFORGE_EDGE_ADDRESS=:8083
TELEMETRYFORGE_EDGE_ID=edge-denver-01
TELEMETRYFORGE_EDGE_WAL_DIR=/var/lib/telemetryforge/wal
TELEMETRYFORGE_EDGE_WAL_SEGMENT_BYTES=67108864
TELEMETRYFORGE_EDGE_WAL_MAX_BYTES=4294967296
```

Run with `make run-edge`. `GET /ready` reflects local durable capacity rather than Kafka reachability. `GET /edge/status` returns segment count, bytes, capacity, pending records, committed sequence, last sequence, lineage signer key ID, sealed-segment count, final record hash, and latest signed segment root.

Verification includes append/reopen recovery, torn-tail truncation, completed-frame corruption rejection, capacity backpressure, sequence continuity, and asynchronous replay after a downstream outage. `make proof-wal-crash-recovery` emits the existing versioned `.tfproof.json` evidence format.

v2.1.0 is the single-node durable-edge foundation. v2.2.0 adds failure-domain-aware peer quorum acceptance. v2.4.0 upgrades new WAL records to cryptographic lineage while retaining v1 record read compatibility. See [Replicated Durability](replicated-durability.md) and [Cryptographic Lineage](../security/cryptographic-lineage.md). Formal verification is delivered in v2.5.0; eBPF collection and autonomous control remain later milestones.
