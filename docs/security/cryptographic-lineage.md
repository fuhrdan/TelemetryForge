# Cryptographic Lineage

TelemetryForge v2.4 adds a signed audit chain to Durable Edge WAL data. It is intentionally not a blockchain: there is no consensus protocol, cryptocurrency, or external ledger.

## Record chain

Every new v2 WAL record contains:

```text
payload_sha256
previous_record_hash
record_hash
```

`record_hash` covers immutable routing/sequencing metadata, the payload digest, and `previous_record_hash`. Changing an event, sequence, topic, acceptance timestamp, or predecessor therefore breaks the chain.

Legacy v1 WAL records are still accepted during upgrade. Their canonical JSON is hashed to create a deterministic anchor, then the edge starts a fresh v2 segment.

## Signed segment seals

When a segment rotates or the edge shuts down cleanly, TelemetryForge:

1. verifies the ordered record chain;
2. uses each record digest as a Merkle leaf;
3. computes a SHA-256 Merkle root;
4. links the seal to the previous segment root;
5. signs the canonical seal metadata with Ed25519; and
6. writes `<segment>.tfseal` beside the WAL.

The seal records the sequence range, first/last record hashes, Merkle root, signer key ID, embedded public key, and signature.

Seals are deliberately retained after delivered WAL segments are compacted. A seal-only historical segment preserves signed continuity, while a seal plus retained `.tfwal` permits full content/Merkle verification.

## Key handling

By default the edge creates these files in its WAL directory:

```text
lineage.ed25519.pem       # private, mode 0600
lineage.ed25519.pub.pem   # public
```

That default is convenient for local development because the WAL volume persists the identity. Production should pre-provision the private key from a secret manager and set:

```text
TELEMETRYFORGE_EDGE_LINEAGE_PRIVATE_KEY=/run/secrets/lineage/edge.pem
TELEMETRYFORGE_EDGE_LINEAGE_PUBLIC_KEY=/run/secrets/lineage/edge.pub.pem
```

Generate a pair with:

```bash
telemetryctl audit keygen \
  --private lineage.ed25519.pem \
  --public lineage.ed25519.pub.pem
```

Do not publish the private key.

## Verification

Integrity-only verification uses the public key embedded in each signed seal:

```bash
telemetryctl audit verify --wal-dir data/edge-wal
```

For identity-authenticated verification, pin the expected edge public key:

```bash
telemetryctl audit verify \
  --wal-dir data/edge-wal \
  --public-key lineage.ed25519.pub.pem
```

The report distinguishes retained content from `seal_only` history and reports the final record hash and segment root.

## Threat boundary

v2.4 provides tamper evidence, not magical compromise resistance. If an attacker can both rewrite historical WAL/seal files and obtain the trusted private key, they can create new valid signatures. Protect the signing key separately from ordinary data access in production.

A signature also does not prove an event was truthful; it proves that the signed edge committed to a specific ordered byte lineage.
