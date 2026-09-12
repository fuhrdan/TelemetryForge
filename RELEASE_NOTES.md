# TelemetryForge v2.4.0 Release Notes

**Release:** Cryptographic Lineage
**Date:** 2026-09-12

v2.4.0 adds a cryptographically verifiable audit lineage to the Durable Edge while preserving the v2.2 durability contract and v2.3 routing semantics. It is intentionally not a blockchain: the implementation uses ordered SHA-256 record chaining, Merkle-rooted WAL segments, and Ed25519 signatures.

## Highlights

- WAL record format v2 with `previous_record_hash` and `record_hash`;
- backward-readable legacy v1 WAL records;
- deterministic upgrade anchoring from v1 into v2 lineage;
- SHA-256 Merkle roots over every closed segment's ordered record digests;
- Ed25519-signed `.tfseal` segment attestations;
- previous-segment-root chaining;
- persistent edge signer identity and key ID;
- default local/dev key generation inside the WAL volume;
- explicit production private/public key paths;
- fail-closed signer mismatch when historical seals already exist;
- signed seal retention after normal WAL segment compaction;
- `telemetryctl audit keygen`;
- `telemetryctl audit verify` with optional trusted-public-key authentication;
- lineage key/root state in `/edge/status`;
- ADR 0055 and `proof/cryptographic-lineage.sh`.

## Record lineage

New records form this chain:

```text
Event payload
    |
    v
payload_sha256
    |
    + previous_record_hash
    + edge/source sequence
    + topic / acceptance metadata
    |
    v
record_hash N
    |
    v
previous_record_hash N+1
```

Changing an event payload, routing metadata, sequence, timestamp, record order, or predecessor breaks verification.

## Segment lineage

On rotation or clean shutdown:

```text
record hashes
    |
    v
SHA-256 Merkle root
    |
    + previous segment root
    + edge identity / sequence range
    |
    v
Ed25519 signature
    |
    v
<segment>.tfseal
```

The `.tfseal` file remains after its delivered `.tfwal` is compacted. If WAL content is still present, verification recomputes the full record chain and Merkle root. If content has aged out, the signed seal still preserves the segment commitment and segment-to-segment continuity.

## Trust modes

Integrity verification:

```bash
telemetryctl audit verify --wal-dir data/edge-wal
```

This verifies each seal using its embedded public key.

Identity-authenticated verification:

```bash
telemetryctl audit verify \
  --wal-dir data/edge-wal \
  --public-key data/edge-wal/lineage.ed25519.pub.pem
```

This additionally requires every seal signer to match the operator-provided trust anchor.

## Upgrade behavior

Existing v2.3 WAL v1 records remain readable. At first v2.4 startup, completed legacy segments are hashed and signed as upgrade-time anchors; new writes begin with WAL record format v2. This proves the legacy bytes observed at upgrade time, not their historical state before v2.4 existed.

## Security boundary

The signing key must be protected independently in production. An attacker who can both rewrite telemetry history and use the trusted private key can create new valid signatures. v2.4 makes unauthorized mutation detectable under the configured key-trust assumptions; it does not claim an external transparency ledger or consensus guarantee.

## Validation

CI is configured to run race/vet/build coverage, the lineage/WAL/replication/mesh/edge suite, Docker/Kubernetes validation, and the manual cryptographic-lineage operational proof scenario under Go 1.27.1.
