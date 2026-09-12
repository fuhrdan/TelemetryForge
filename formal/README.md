# TelemetryForge Formal Models

TelemetryForge v2.5.0 adds bounded formal models for the safety boundaries that
matter most to the Durable Edge. The models intentionally abstract away HTTP,
Kafka clients, storage encodings and timing. Their job is to make the protocol
claims precise enough to model-check.

The repository carries two executable layers:

1. **TLA+ / TLC** specifications in this directory with finite model configs in
   `formal/models/`.
2. A dependency-free Go state-space checker in `internal/formal`, exposed as
   `go run ./cmd/formalcheck`. It mirrors the same release invariants and runs in
   the normal Go CI job even when TLC is unavailable.

## Safety invariants

| Model | Key invariants |
|---|---|
| `DurableIngest` | client success has durable evidence; acked undelivered data remains durable; no early compaction |
| `ReplicatedDurability` | replicated success requires quorum; quorum contains the origin; replicas release only after downstream delivery |
| `MeshFailover` | an owner is attempted at most once per delivery; terminal failure means no eligible route; success and terminal failure cannot coexist |
| `CryptographicLineage` | tampered sealed history cannot verify; verified history is untampered; seals commit a non-empty chain |

These are **safety** models. They do not prove that a failed network eventually
recovers, that Kafka eventually accepts an event, or that an arbitrary cloud
outage ends. Liveness depends on environmental fairness assumptions and is kept
out of the v2.5 claim boundary.

## Run the bounded Go checker

```bash
go run ./cmd/formalcheck
```

To save machine-readable evidence:

```bash
go run ./cmd/formalcheck --output formal-check.json
```

## Run TLC

Install a compatible `tla2tools.jar`, then set `TLA2TOOLS_JAR`:

```bash
export TLA2TOOLS_JAR=/path/to/tla2tools.jar
scripts/run-tlc.sh
```

CI uses the declared TLA+ tools release in `formal/tla2tools.version` and verifies the downloaded JAR against `formal/tla2tools.sha256`. The v1.8.0 upstream tag is a rolling prerelease channel, so a byte change fails CI until the repository digest is deliberately reviewed and updated.

## Scope and claim discipline

A successful model check means no invariant violation exists in the explored
finite state space under the specification. It does **not** prove the Go
implementation is bug-free or that hardware cannot violate the durability
assumptions. Implementation tests, race tests, operational proof, and TLA+
serve different evidence roles and are intentionally kept together.
