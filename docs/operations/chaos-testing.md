# Failure / Chaos Testing

The v1.9 proof harness deliberately exercises broker, database, worker, and
connector failures. Every disruptive script requires `--execute`.

Run only in an environment where stopping a service is acceptable. Preserve the
resulting `.tfproof.json` and its raw evidence before interpreting a recovery
number. A failed proof is useful evidence and should not be rewritten as pass.

See [HA / Failure Proof Scenarios](../proof/ha-scenarios.md).
