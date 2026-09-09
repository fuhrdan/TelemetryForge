# Incident Replay

Incident Replay reprocesses a **frozen Incident Flight Recorder window** through
current or candidate policy without sending that telemetry back through the
normal production pipeline.

## Why replay exists

A policy or processing change is safer to review against real incident evidence
than against invented samples.

Replay answers questions such as:

- Would the current Cardinality Firewall drop different dimensions?
- Would a candidate shadow policy quarantine more events?
- How many events change under the new rules?
- How many findings or active/shadow disagreements appear?

## Default safety boundary

The default replay is analysis-only.

```text
frozen incident
      |
      v
normalizer
      |
      v
active + shadow policy
      |
      v
compact replay results/history
```

It does **not** call:

- the production ingestion endpoint;
- the normal primary telemetry persister;
- the Flight Recorder;
- automatic incident detection; or
- a Kafka publisher.

This prevents a replay from accidentally looking like fresh production traffic.

## Run a replay

```bash
telemetryctl incident replay \
  --id INC-2026-0042 \
  --policy policies/active.json \
  --shadow-policy policies/shadow.json
```

The run stores:

- incident ID
- active/shadow policy versions
- event count
- changed-event count
- dropped-tag count
- quarantine count
- cardinality-finding count
- shadow-difference count
- status/error
- optional isolated publish count

Per-event results contain only effect metadata, not another copy of the
telemetry payload.

## Optional Kafka output

An operator can explicitly publish replay output to an isolated replay topic:

```bash
telemetryctl incident replay \
  --id INC-2026-0042 \
  --publish-topic telemetry.replay
```

`telemetryctl` rejects normal production ingest topics. The replay runner itself
also enforces the replay-topic namespace.

Published replay events are labeled with:

```text
telemetryforge.replay_run
telemetryforge.replay_incident
```

No checked-in worker consumes `telemetry.replay`.

## Replay and event time

Policy/cardinality evaluation uses the frozen event timestamps for replay
observation time rather than the wall clock of the replay command.

That makes growth windows and rate-limited findings represent the incident
timeline instead of how quickly a laptop can replay it.

## Dashboard

Replay history is readable through:

```text
GET /api/v1/replays
```

and appears under **Incident Replay** in the TelemetryForge dashboard.
