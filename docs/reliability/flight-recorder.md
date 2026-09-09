# Incident Flight Recorder

The Incident Flight Recorder is one of TelemetryForge's signature features.

v0.5.0 introduces its storage foundation.

## The problem

Sampling and retention policies are useful until the discarded telemetry turns
out to be the one record needed to explain an outage.

The Flight Recorder keeps a short rolling copy of the full incoming telemetry
envelope before normal processing changes it.

## v0.5.0 behavior

Every decoded event passes through:

```text
Kafka
  |
  v
Flight Recorder
  |
  v
Normalizer
  |
  v
Persistent telemetry store
```

The rolling buffer is stored in the `flight_recorder_events` TimescaleDB
hypertable and has a 30-minute development retention policy.

It is intentionally separate from the main telemetry table.

## Freezing an incident

An operator can preserve a time window before the rolling buffer expires:

```bash
telemetryctl incident freeze \
  --id INC-2026-0042 \
  --title "Checkout latency spike" \
  --from 2026-09-09T16:00:00Z \
  --to   2026-09-09T16:20:00Z
```

TelemetryForge copies matching captured events into durable `incident_events`.

The source rolling buffer can then expire normally.

## Why capture before normalization?

The Flight Recorder should answer:

> What did the processing pipeline actually receive?

If normalization or a future policy introduces a bug, recording only the
post-processed event could hide the evidence.

## What this enables later

The frozen incident model is the foundation for:

- incident replay
- policy testing against historical traffic
- "would this alert have fired?" analysis
- Evidence Graph construction
- incident export/import packages
- training scenarios
