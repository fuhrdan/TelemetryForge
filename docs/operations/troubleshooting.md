# Troubleshooting

## `docker compose up --build` does not become healthy

Check all service states:

```bash
docker compose ps
```

Then inspect the failing service:

```bash
docker compose logs --tail=200 kafka
docker compose logs --tail=200 timescaledb
docker compose logs --tail=200 gateway
docker compose logs --tail=200 worker
docker compose logs --tail=200 dashboard
```

## Gateway `/ready` returns 503

The gateway readiness endpoint currently depends on Kafka connectivity. Verify
Kafka is healthy and that `TELEMETRYFORGE_KAFKA_BROKERS` uses an address
reachable from the gateway's network namespace.

Inside Compose, use `kafka:19092`. From the host, use `localhost:9092`.

## Worker starts but events do not appear in the database

Inspect the consumer group:

```bash
make kafka-groups
```

Then inspect worker logs and recent stored rows:

```bash
docker compose logs --tail=200 worker
make db-events
```

Growing consumer lag with stable worker memory usually means backpressure is
working and processing/storage is slower than ingestion.

## Database migration fails

Run the migration service directly to get unbuffered output:

```bash
docker compose run --rm db-migrate
```

Migrations are intended to be idempotent. Do not manually edit a production
schema to work around a failed migration without understanding the mismatch.

## Dashboard loads but shows no live events

Check the Go API first:

```bash
curl http://localhost:8080/api/v1/dashboard/summary?window=5m
```

Generate traffic:

```bash
make demo-traffic
```

The browser reaches the API through the Next.js `/telemetry-api/` rewrite. If
the dashboard runs outside Compose, its API base should point at
`http://localhost:8080`.

## Automatic incident does not appear

Generate the deliberate incident demo:

```bash
make demo-incident
```

Then inspect worker logs and the incident API:

```bash
curl http://localhost:8080/api/v1/incidents
```

Remember that duplicate automatic triggers for the same source/reason category
are suppressed by the cooldown.

## DLQ inspection

```bash
make dlq-tail
```

A source record is committed only after its terminal failure has been accepted
by the DLQ path.

## Reset local data

This destroys the local TimescaleDB volume:

```bash
make docker-down
```

`make docker-down` uses `docker compose down -v`; do not use it against data you
intend to keep.
