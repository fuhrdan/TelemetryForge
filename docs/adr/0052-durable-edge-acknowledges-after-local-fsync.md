# ADR 0052: Durable edge acknowledges after local fsync

## Status
Accepted for v2.1.0.

## Context
The existing gateway acknowledges after Kafka's configured broker acknowledgement policy. The Global Edge Fabric roadmap needs an optional ingestion boundary that can remain safe during a temporary broker/network outage.

## Decision
`telemetryforge-edge` writes every accepted event to a local segmented WAL and calls `fsync` before successful acceptance. Kafka delivery happens asynchronously in global edge-sequence order. A delivery checkpoint advances only after downstream acknowledgement.

A crash can therefore replay an already delivered but not checkpointed event, preserving at-least-once behavior. When WAL capacity is exhausted, the edge rejects new acceptance and never overwrites pending accepted records. A partial final frame is recoverable as an interrupted write; integrity failure in a complete frame fails startup closed.

## Consequences
Temporary Kafka outages no longer have to make edge ingestion immediately unavailable; local durable storage becomes part of the acceptance SLO; duplicate downstream delivery remains possible after crash/recovery; single-node storage failure remains a durability boundary until later replication work.
