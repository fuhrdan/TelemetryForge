# Failure Handling in v0.3.0

v0.3.0 establishes the rules that later retry and dead-letter features build on.

## Processing failure

If a processor returns an error, the worker logs the failure and does not
commit the Kafka record. This protects against silently losing a record.

## Invalid JSON from Kafka

A malformed record is logged with its topic, partition, and offset. v0.3.0 does
not yet move poison records to a dead-letter topic. That is deliberate: DLQ
metadata, retry ceilings, and replay semantics will be implemented together so
they form one coherent reliability contract.

## Process shutdown

SIGINT and SIGTERM cancel polling. The worker stops taking new Kafka work,
finishes jobs already placed in the bounded queue, and then closes its Kafka
client.

## Delivery guarantee

The current contract is at-least-once. A worker can finish a side effect and
crash before committing the offset, causing the record to be processed again.
Future persistence code must use event IDs as idempotency keys.
