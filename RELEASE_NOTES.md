# TelemetryForge v2.5.0 Release Notes

**Release:** Formal Verification
**Date:** 2026-09-12

v2.5.0 makes the Durable Edge protocol claims executable. The release adds TLA+ safety specifications, finite TLC model configurations, a dependency-free Go state-space checker, property-based schedule fuzzing, and proof/CI integration for the durability, replication, routing, and lineage boundaries delivered in v2.1-v2.4.

## Highlights

- `formal/DurableIngest.tla` models fsync, acknowledgement, delivery, and compaction;
- `formal/ReplicatedDurability.tla` models origin persistence, peer quorum, delivery, and release;
- `formal/MeshFailover.tla` models bounded owner attempts and no-route termination;
- `formal/CryptographicLineage.tla` models segment sealing, tamper, verification, and rejection;
- finite TLC configurations under `formal/models/`;
- SANY syntax validation before each TLC run;
- pinned TLA+ tool version in `formal/tla2tools.version`;
- `internal/formal` dependency-free exhaustive checker;
- `cmd/formalcheck` human and JSON output;
- adversarial Go fuzz target for durability action schedules;
- dedicated CI formal-verification job;
- `proof/formal-verification.sh` and `.tfproof.json` integration;
- ADR 0056 and formal proof-boundary documentation.

## Checked safety properties

### Durable ingest

```text
client acknowledgement
        => durable evidence exists

acked + not delivered
        => local WAL remains durable

compacted
        => downstream delivery occurred
```

### Replicated durability

```text
replicated acknowledgement
        => configured quorum exists

acked + not delivered
        => quorum remains present

release replicas
        => downstream delivery occurred
```

### Mesh failover

Each candidate owner can be attempted at most once in one abstract delivery path. Terminal routing failure requires the eligible candidate set to be empty, so the model cannot represent recursive edge-to-edge bouncing as a successful failover strategy.

### Cryptographic lineage

A sealed history may verify only while the committed chain remains untampered in the model. Post-seal mutation transitions into rejection, never successful verification.

## Dual model-checking path

The normal Go toolchain can run:

```bash
go test ./internal/formal
go run ./cmd/formalcheck
```

The full formal CI job additionally runs all TLA+ models through SANY and TLC with the pinned TLA+ tools release.

The operational proof requires both layers and stores their logs/report as evidence:

```bash
TLA2TOOLS_JAR=/path/to/tla2tools.jar \
  proof/formal-verification.sh --execute
```

## Proof scope

This release deliberately does not call the result “mathematically proven zero loss.” The checked models establish bounded safety properties under the model assumptions. They do not prove that networks recover, hardware never destroys all copies, downstream systems eventually respond, or the production implementation contains no unrelated defect.

Formal models, implementation tests, race/fuzz checks, integration tests, cryptographic audit verification, and operational proof remain separate evidence layers.
