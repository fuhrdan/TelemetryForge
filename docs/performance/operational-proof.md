# Operational Proof Artifacts

TelemetryForge v1.9.0 introduces `.tfproof.json`, a versioned evidence format for
HA, recovery, backup/restore, connector isolation, rolling-upgrade, and load
runs.

A proof artifact is not a marketing claim. It records what was executed, which
assertions passed, which measurements were observed, where raw evidence lives,
and the SHA-256 fingerprints of the configuration used for the run.

## Required evidence

A valid proof contains:

- `run_id` and scenario;
- `pass`, `fail`, or `error` status;
- start/completion timestamps;
- at least one explicit assertion;
- at least one evidence reference;
- at least one configuration fingerprint.

A `pass` artifact cannot contain a failed assertion. Measurement values must be
finite numbers; `NaN`/`Infinity` are rejected.

## Verify

```bash
telemetryctl proof verify --file proof/results/run.tfproof.json
```

The verifier returns the parsed artifact plus the SHA-256 and byte count of the
exact file.

## Record

```bash
telemetryctl proof record --file proof/results/run.tfproof.json
```

Database recording is idempotent only when the same `run_id`, whole-file
SHA-256, and byte count are presented again. Reusing a run ID with different
bytes is rejected.

## Read

```bash
telemetryctl proof list --limit 25
```

API:

```text
GET /api/v1/proofs
```

The dashboard renders the recorded status, measurements, assertion counts,
artifact hash, and recorded time.

## Claims policy

Do not turn a harness exit code into an unstated performance claim. A measured
throughput/RTO/recovery number may be published only when the artifact or its
referenced raw evidence actually contains that measurement and the test
configuration is fingerprinted.
