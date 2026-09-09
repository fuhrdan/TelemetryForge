# Upgrade to v1.0.0

v1.0.0 introduces tenant-aware storage.

## Before upgrade

1. Back up PostgreSQL/TimescaleDB.
2. Record the running TelemetryForge tag.
3. Preserve current policy files.
4. Confirm Kafka consumer lag is drained or understood.
5. Confirm no migration is already running.

## Migration behavior

`007_security_evidence_graph.sql` assigns existing pre-v1 rows to:

```text
tenant_id = default
```

The local disabled-auth profile also uses `default`, so existing data remains
visible after a local upgrade.

Event deduplication changes from:

```text
event_id
```

to:

```text
tenant_id + event_id
```

## Production auth rollout

A safe rollout order is:

1. deploy v1 with auth **disabled** on a private network;
2. run database migrations;
3. verify existing `default` tenant data;
4. prepare hashed API-key/secret documents;
5. deploy dashboard server key;
6. switch gateway `TELEMETRYFORGE_AUTH_MODE=api_key`;
7. verify ingest/read/admin keys independently;
8. enable production Kafka TLS/SASL profile;
9. validate authenticated Prometheus scraping.

## Validation

Check:

```text
GET /health
GET /ready
```

Then authenticated:

```text
POST /api/v1/events
GET /api/v1/dashboard/summary
GET /metrics
```

Confirm:

- read-only key cannot ingest;
- ingest key cannot read dashboard history;
- tenant A cannot read tenant B;
- client-supplied tenant_id is rejected;
- dashboard SSE remains live through the server proxy.
