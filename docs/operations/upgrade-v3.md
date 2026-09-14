# Upgrade to v3.0.0

TelemetryForge v3.0.0 is designed as a consolidation release rather than a destructive storage/API migration.

## Before upgrading

1. Record the current commit and active policy/routing/shaping configuration.
2. Run the existing WAL and cryptographic-lineage verification for edge nodes.
3. Preserve a database backup and any required `.tfincident` / `.tfproof.json` artifacts.
4. Confirm the configured durability mode and peer failure-domain metadata.

## Compatibility

v3.0 retains the existing public `/api/v1` surface, internal `/internal/v1` replication/mesh surface, WAL v1/v2 read compatibility, and current evidence archive/proof formats. No database migration is introduced solely for the fabric status contract.

## Rollout

Roll edge nodes one failure domain at a time. After each node returns, inspect:

```bash
curl -s http://EDGE:8083/edge/fabric/status
```

A normal fully connected node should report:

```text
state=ready
acceptance_ready=true
delivery_ready=true
audit_ready=true
```

A downstream outage may legitimately report `degraded` while `acceptance_ready=true`. Do not treat that state as evidence that pending WAL data can be discarded.

`not_ready` means the configured acceptance contract is unavailable; investigate WAL pressure, replication quorum, lineage identity, or a required eBPF collector before restoring ingestion traffic.

## Rollback

Because v3.0 does not replace the WAL format, a rollback to the immediately preceding v2.9 runtime can read existing v2 WAL data. Preserve signed seal/key material and do not delete pending WAL/replica state during rollback.
