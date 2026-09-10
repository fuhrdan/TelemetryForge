# Incident Archive Workflow

This runbook covers export, verification, transfer, offline investigation, and
import.

## 1. Freeze/select an incident

Archives are built from frozen Incident Flight Recorder evidence.

```bash
telemetryctl incident graph --id INC-42 --tenant production
```

is a useful pre-export check but not required; export regenerates the Evidence
Graph from the frozen evidence and current stored replay/cost history.

## 2. Export

Unencrypted:

```bash
telemetryctl incident export \
  --id INC-42 \
  --tenant production \
  --out incident-exports/INC-42.tfincident
```

Encrypted:

```bash
telemetryctl incident export \
  --id INC-42 \
  --tenant production \
  --out incident-exports/INC-42.tfincident.enc \
  --encrypt-key-file /secure/path/archive.key
```

Export will not overwrite an existing file unless `--force` is explicit.

## 3. Record the digest

The command prints the complete-file SHA-256.

Record it somewhere independent of the file when chain-of-custody matters.

## 4. Verify before transfer

```bash
telemetryctl incident verify --file incident-exports/INC-42.tfincident
```

For encryption, supply the key file.

## 5. Investigate offline

Summary:

```bash
telemetryctl incident inspect --file incident-exports/INC-42.tfincident
```

Standalone report:

```bash
telemetryctl incident report \
  --file incident-exports/INC-42.tfincident \
  --out incident-exports/INC-42.html
```

Neither requires PostgreSQL or Kafka.

## 6. Transfer securely

For real production evidence:

- prefer encrypted archives;
- transfer the AES key through a separate approved channel;
- verify the whole-file SHA-256 at the destination;
- control access to generated HTML reports as carefully as the archive.

## 7. Import

Same tenant:

```bash
telemetryctl incident import \
  --file INC-42.tfincident
```

Different target incident ID:

```bash
telemetryctl incident import \
  --file INC-42.tfincident \
  --new-id INC-42-LAB
```

Explicit cross-tenant/environment remap:

```bash
telemetryctl incident import \
  --file INC-42.tfincident \
  --tenant lab \
  --allow-tenant-remap
```

Without `--allow-tenant-remap`, an archive cannot silently cross its recorded
tenant boundary.

## Import semantics

Import:

- inserts a frozen incident;
- rewrites embedded event tenant IDs to the explicitly selected trusted target;
- records archive provenance;
- regenerates/stores an Evidence Graph for the target incident.

Import does **not**:

- insert events into normal primary telemetry;
- run adaptive sampling;
- run Cardinality Firewall policy;
- route to external destinations;
- activate archived configuration;
- copy archived replay/cost rows into live history.

Replay/cost/configuration artifacts remain available inside the portable file.
They are evidence, not executable deployment state.

## Duplicate protection

A target incident ID that already exists fails.

The same archive ID cannot be imported twice into the same tenant. This keeps
the provenance table unambiguous.
