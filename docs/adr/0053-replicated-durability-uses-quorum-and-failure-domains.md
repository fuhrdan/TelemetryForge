# ADR 0053: Replicated durability uses quorum and failure domains

**Status:** Accepted for v2.2.0.

## Context

v2.1.0 introduced a local fsync acceptance boundary. A successful local write survives process/Kafka failure, but a permanent loss of that edge disk can still destroy the only durable copy.

Counting peer acknowledgements alone is not enough to strengthen the guarantee. Two copies on the same host, availability zone, region, or cloud may fail together.

## Decision

TelemetryForge v2.2.0 adds synchronous edge replication before successful non-local acceptance.

The local WAL write always happens first. The configured durability mode then evaluates durable acknowledgements across explicit failure-domain metadata:

- `local`: one local fsynced copy;
- `regional`: quorum plus at least two zones in the local region;
- `cross-region`: quorum plus at least two regions;
- `cross-cloud`: quorum plus at least two clouds.

Quorum counts the local durable copy. Receiver peers acknowledge only after their replica log has been fsynced. Replica writes retain the origin `edge_id` and `edge_sequence`, are idempotent for exact retries, and reject conflicting records for an already-seen origin sequence.

If quorum cannot be reached, the request returns failure. The locally fsynced record remains in the origin WAL and the replay loop retries replication before downstream Kafka delivery. Therefore a failed client request can be indeterminate: it may later satisfy quorum and be delivered. Clients that retry should retain stable event IDs so the existing tenant/event deduplication boundary can collapse duplicate delivery.

Replication traffic uses a separate bearer token from public tenant authentication. Production deployments should use HTTPS/mTLS-capable network boundaries in addition to the shared token.

## Consequences

- non-local successful acceptance now has an explicit multi-node durability boundary;
- placement metadata becomes part of the durability contract;
- losing quorum intentionally removes readiness and successful acceptance instead of silently weakening durability;
- replica storage introduces additional disk/network capacity requirements;
- peer writes can be repeated during replay because they are idempotent;
- v2.2 does not yet provide consensus membership, automatic topology discovery, cryptographic lineage, or formal proof of the protocol.
