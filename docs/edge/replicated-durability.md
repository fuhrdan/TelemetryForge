# Replicated Durability

TelemetryForge v2.2.0 extends the v2.1 local Durable Edge WAL with synchronous peer replication and failure-domain-aware quorum acceptance.

```text
Client
  |
  v
Origin Edge
  |
  +--> local WAL append + fsync ------------------------+
  |                                                     |
  +--> Edge peer B -> replica log + fsync -> ACK        | quorum
  |                                                     |
  `--> Edge peer C -> replica log + fsync -> ACK -------+
                                                        |
                                             HTTP success
                                                        |
                                                        v
                                            ordered Kafka replay
```

The origin WAL is always written first. For any mode stronger than `local`, successful client acceptance additionally requires the configured quorum and failure-domain rule.

## Durability modes

| Mode | Acceptance requirement |
|---|---|
| `local` | local WAL fsync |
| `regional` | quorum plus durable copies in at least two zones of the local region |
| `cross-region` | quorum plus durable copies in at least two regions |
| `cross-cloud` | quorum plus durable copies in at least two clouds |

`TELEMETRYFORGE_EDGE_REPLICATION_QUORUM` counts the origin's local WAL copy. A regional quorum of `2`, for example, means the origin plus at least one peer in a different zone of the same region.

An acknowledgement is not counted until the receiving peer validates the origin WAL record, appends it to its crash-safe replica log, and fsyncs the file. Repeated delivery of the same `(origin edge ID, edge sequence)` is idempotent. A different record presented for an existing origin sequence is rejected as a conflict. Replica storage is bounded; a full peer returns insufficient-storage instead of acknowledging a write it could not durably retain.

After the origin checkpoints downstream Kafka delivery, it sends a durable release-through checkpoint to peers. Peers can then discard those replica records and compact their logs. Release failure is safe: it retains extra copies and is retried later rather than deleting data early.

## Failure behavior

When quorum is unavailable:

1. the origin WAL record remains durable locally;
2. the request returns HTTP 503 through the normal ingestion API;
3. `/ready` becomes unavailable while the required live failure-domain policy cannot be satisfied;
4. the replay loop retries replication before sending the record to Kafka;
5. once quorum returns, ordered downstream delivery resumes.

A failed HTTP request after local persistence is therefore **indeterminate**, not proof that the event was discarded. Producers should retry with a stable event ID. TelemetryForge's existing `(tenant_id,event_id)` storage idempotency prevents cross-retry duplicate processing at the persistence boundary.

## Configuration

```bash
TELEMETRYFORGE_EDGE_ID=edge-denver-a
TELEMETRYFORGE_EDGE_CLOUD=aws
TELEMETRYFORGE_EDGE_REGION=us-west-2
TELEMETRYFORGE_EDGE_ZONE=us-west-2a

TELEMETRYFORGE_EDGE_DURABILITY_MODE=regional
TELEMETRYFORGE_EDGE_REPLICATION_QUORUM=2
TELEMETRYFORGE_EDGE_REPLICATION_TIMEOUT=3s
TELEMETRYFORGE_EDGE_REPLICATION_TOKEN=replace-with-secret-manager-value

TELEMETRYFORGE_EDGE_PEERS='edge-denver-b|https://edge-b.internal:8083|aws|us-west-2|us-west-2b,edge-denver-c|https://edge-c.internal:8083|aws|us-west-2|us-west-2c'
TELEMETRYFORGE_EDGE_REPLICA_DIR=/var/lib/telemetryforge/replicas
TELEMETRYFORGE_EDGE_REPLICA_MAX_BYTES=8589934592
```

Peer syntax is:

```text
id|url|cloud|region|zone
```

The non-local modes require `TELEMETRYFORGE_EDGE_REPLICATION_TOKEN`. The internal endpoints are:

```text
GET  /internal/v1/replication/health
POST /internal/v1/replicate
```

They use the replication bearer token and are intentionally separate from public tenant API authentication.

## Docker Compose demo

The repository Compose topology starts three edge nodes. The public `edge` service uses `regional` durability, quorum `2`, and peers in logical zones `b` and `c`.

```bash
docker compose up -d --wait kafka kafka-init edge-peer-b edge-peer-c edge
curl -s http://localhost:8083/edge/status | jq
```

Stop both peers and readiness will fail rather than silently falling back to local-only durability. Restore either peer and a 2-of-3 regional quorum can be satisfied again.

## Scope

v2.2 provides failure-domain-aware replicated acceptance; it is not a consensus system. Peer membership is static configuration, replica logs are receiver-side recovery copies rather than an automatic new origin, and no claim is made that every possible infrastructure disaster is survivable.

Formal verification, cryptographic lineage, dynamic global routing, and automated replica promotion remain later milestones.
