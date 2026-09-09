# v1.0.0 Rollback Runbook

Database migrations are additive except for tenant-scoping changes to selected
primary keys. Treat rollback as an operational event, not simply `git checkout`.

## Preferred application rollback

If the v1 application fails after migration:

1. keep the v1 database schema;
2. stop v1 gateway/workers;
3. deploy the previous application only if it has been validated against the
   migrated schema in your environment;
4. otherwise restore the pre-upgrade database backup with the previous release.

## Why a blind SQL downgrade is not provided

v1 can accept multiple tenants using the same event ID.

Collapsing `(tenant_id,event_id)` back to globally unique `event_id` can lose or
conflict with valid data.

TelemetryForge therefore does not ship a destructive automatic "down migration"
that pretends the transformation is always reversible.

## Auth emergency rollback

If authentication configuration is the failure and the service remains on a
trusted private network, an operator can temporarily set:

```text
TELEMETRYFORGE_AUTH_MODE=disabled
```

while correcting the key file.

Do not use this as an Internet-facing recovery shortcut.

## Kafka security rollback

TLS/SASL settings can be reverted independently if the broker still permits the
previous transport mode.

Prefer correcting credentials/certificates rather than weakening broker policy.

## Preserve incident evidence

Before destructive recovery, export or back up any new frozen incidents/replay
results needed for postmortem analysis.
