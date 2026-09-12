# ADR 0055: Cryptographic lineage uses hash chains, Merkle roots, and Ed25519

**Status:** Accepted for v2.4.0.

## Context

The Durable Edge can prove that accepted events survived crash/replay, but CRC32 and payload SHA-256 alone do not provide an ordered, signed audit trail. TelemetryForge needs tamper evidence that remains understandable without introducing a distributed-consensus ledger.

## Decision

1. New WAL records use format version 2 and include the SHA-256 hash of the previous record plus a deterministic record hash.
2. Legacy v1 records remain readable and are deterministically hashed when an upgraded edge establishes its first lineage anchor.
3. Closed WAL segments build a binary SHA-256 Merkle tree over ordered record digests.
4. Each segment seal contains the Merkle root, first/last record hashes, sequence range, previous segment root, edge identity, and timestamp.
5. Segment seals are signed with Ed25519. The edge key ID is the first eight bytes of SHA-256 over the public key.
6. Seals are retained after WAL data compaction so the signed segment-root chain survives normal delivery cleanup.
7. `telemetryctl audit verify` verifies embedded signatures; supplying `--public-key` additionally authenticates the signer against an operator trust anchor.
8. A missing or mismatched signing key fails edge recovery when existing seals are present rather than silently creating a new identity.

## Consequences

- Bit changes to retained records, record order, segment membership, seal metadata, or signatures are detectable.
- A trusted public key turns integrity verification into edge-identity authentication.
- A retained seal without its compacted WAL can prove signed historical continuity but cannot independently reconstruct the deleted event payloads.
- Upgrade-time sealing of legacy v1 WAL proves the state observed at upgrade time; it cannot retroactively prove how those bytes were created before v2.4.
- Key rotation requires an explicit authenticated transition mechanism and is deferred.
