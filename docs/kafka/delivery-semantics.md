# Kafka Delivery Semantics

TelemetryForge uses Kafka as the durable boundary between ingestion and
processing. The current contract is **at least once** end to end.

## Producer acceptance

The gateway returns `202 Accepted` only after Kafka acknowledges the produced
record.

```text
HTTP request
    |
    v
validate envelope
    |
    v
produce to Kafka
    |
    v
wait for broker acknowledgement
    |
    +-- failure --> HTTP 503
    |
    `-- success --> HTTP 202
```

The producer requests acknowledgements from all in-sync replicas and keeps
franz-go's idempotent producer behavior enabled.

## Consumer acknowledgement

Consumer auto-commit is disabled.

A normal record is considered handled only after:

1. Flight Recorder capture succeeds;
2. normalization succeeds;
3. TimescaleDB persistence succeeds idempotently; and
4. any required processing stages complete.

A terminal processing failure is considered handled only after its detailed
dead-letter replacement is acknowledged by `telemetry.dlq`.

Only then can the source partition offset advance.

## Concurrent workers and partition order

Workers run concurrently, so offset 11 can finish before offset 10 even though
both came from the same Kafka partition.

Kafka commits represent a **partition position**, not independent record
checkboxes. Committing 11 while 10 is unfinished could cause 10 to disappear
after a crash.

TelemetryForge therefore registers records in poll order and advances a
partition commit only through its contiguous completed prefix.

```text
partition 2

offset 10    processing
offset 11    done
offset 12    done

safe commit: none

offset 10    done
offset 11    done
offset 12    done

safe commit: through offset 12
```

Different partitions can advance independently.

## Consumer-group rebalances

franz-go consumer groups can rebalance while application code is processing
records. A commit after ownership has moved can create duplicate or rewind
behavior.

TelemetryForge uses `BlockRebalanceOnPoll` with bounded `PollRecords` batches.
A non-empty batch keeps its current partition ownership until every submitted
record in that batch has finished normal or dead-letter handling. The consumer
then calls `AllowRebalance` before polling the next batch.

The trade-off is important: processing a batch must remain comfortably below
the Kafka rebalance timeout. The batch size is deliberately capped at 100
records, retries are bounded, and v0.7.0 observability/scaling work should expose
processing time and lag so this assumption is measurable.

## Crash behavior

A crash can replay work whose database write succeeded but whose Kafka commit
did not. Event-ID idempotency turns that duplicate delivery into a safe
persistence no-op.

TelemetryForge chooses this duplicate possibility over silently losing
telemetry.

## Local versus production replication

The local single-broker Docker environment uses replication factor 1 because no
second broker exists.

A production deployment should use a replicated Kafka cluster, topic
replication appropriate to its availability target, authenticated/encrypted
broker connections, and monitored consumer lag.

See also:

- `docs/kafka/partitioning.md`
- `docs/kafka/topic-strategy.md`
- `docs/adr/0006-manual-offset-commit.md`
- `docs/adr/0013-coordinate-concurrent-kafka-acknowledgements.md`
