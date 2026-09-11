# HA / Failure Proof Scenarios

The `proof/` directory contains intentionally disruptive harnesses. They refuse
to run unless `--execute` is supplied. Kubernetes rolling restart additionally
requires `--ack-cluster-change`.

## Kafka outage and recovery

```bash
proof/kafka-outage.sh --execute
```

Stops Kafka, observes readiness behavior, restarts the broker, waits for the
gateway to become ready, and records measured recovery seconds.

## PostgreSQL outage and recovery

```bash
proof/postgres-outage.sh --execute
```

Stops TimescaleDB, restores it, and measures time until gateway readiness
returns.

## Worker failover

```bash
proof/worker-failover.sh --execute
```

Starts multiple workers, stops one, and verifies another remains running.

## Connector outage isolation

```bash
proof/connector-outage.sh --execute
```

Uses `routing/proof-connector-outage.json` to fan out one proof event to a
healthy Kafka destination and a deliberately unreachable HTTP connector. The
proof passes only if the healthy lane delivers, the broken lane retries or
dead-letters, and router readiness remains healthy.

## Database backup / restore

```bash
proof/backup-restore.sh --execute
```

Creates a real `pg_dump`, restores it into a temporary database, and compares
selected durable row counts. The temporary database is isolated from the source
DB.

## Kubernetes rolling restart

```bash
proof/k8s-rollout.sh --execute --ack-cluster-change
```

Restarts gateway, worker, router, and dashboard Deployments and waits for each
rollout to complete.

## Load scenarios

```bash
proof/run-k6.sh smoke --execute
proof/run-k6.sh sustained --execute
proof/run-k6.sh backpressure --execute
```

The proof records the actual k6 run result and wall duration. Raw k6 output is
kept as referenced evidence. No throughput or capacity number is invented by
the wrapper.
