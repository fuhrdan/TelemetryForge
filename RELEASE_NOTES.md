# TelemetryForge v0.5.0 Release Notes

## Reliability, DLQ & Incident Flight Recorder

v0.5.0 turns failure handling into a first-class part of the architecture and
introduces the first TelemetryForge-specific incident feature.

### Added

- transient/permanent processing-error classification
- bounded exponential retries with 20% jitter
- four total attempts by default
- Kafka `telemetry.dlq` topic
- detailed dead-letter envelope
- malformed-payload dead-letter preservation
- original Kafka topic/partition/offset metadata
- source-offset acknowledgement after successful DLQ publication
- `telemetryctl` operations CLI
- DLQ replay from a dead-letter JSON record
- rolling 30-minute Incident Flight Recorder
- durable incident freezing
- deduplication pruning command
- explicit database migration service for existing volumes
- retry and terminal-failure tests
- Flight Recorder tests
- reliability documentation and ADRs

### Incident Flight Recorder

The worker now records the incoming canonical envelope before normalization.

The rolling TimescaleDB buffer keeps 30 minutes by default. Operators can freeze
a selected capture window into durable incident storage:

```bash
telemetryctl incident freeze \
  --id INC-2026-0042 \
  --title "Checkout latency spike" \
  --from 2026-09-09T16:00:00Z \
  --to 2026-09-09T16:20:00Z
```

This creates the persistence model needed by later Incident Replay and Evidence
Graph releases.

### Retry semantics

Transient errors retry with bounded exponential backoff. Permanent failures do
not waste repeated attempts.

After four total failed attempts, transient work moves to the DLQ.

Unknown/unclassified errors default to permanent to avoid accidental infinite
retry behavior.

### DLQ acknowledgement semantics

The original Kafka record is not committed until Kafka acknowledges the
dead-letter record.

If DLQ publication fails, the original record remains available to the consumer
group.

### Upgrade behavior

v0.5.0 adds an explicit `db-migrate` service. This fixes an important local
upgrade concern: PostgreSQL init scripts only execute on an empty database
volume, while the migration service applies idempotent migrations to existing
v0.4.0 volumes as well.

### Known limitations

- DLQ browsing UI is not implemented yet.
- Replay currently operates on an exported dead-letter JSON record.
- Flight Recorder freezing is manual; automatic anomaly-triggered freezing
  arrives later.
- Flight Recorder uses the same TimescaleDB instance in this development
  architecture; a cheaper/local buffer may replace it at larger scale.
- retry classification is intentionally small and will become richer as more
  downstream dependencies are introduced.
- authentication, authorization, tenant isolation, and PII redaction remain
  future milestones.
