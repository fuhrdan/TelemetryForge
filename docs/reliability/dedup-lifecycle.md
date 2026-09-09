# Deduplication Lifecycle

At-least-once processing requires TelemetryForge to remember which event IDs
have already been durably stored.

That memory cannot grow forever.

## Current retention relationship

Raw telemetry defaults to 30 days.

The `telemetryctl dedup prune` command defaults to keeping deduplication IDs for
35 days:

```bash
telemetryctl dedup prune
```

The extra five days provide a safety margin beyond the raw telemetry retention
window.

The current CLI refuses a dedup prune horizon shorter than 30 days.

## Why not delete dedup IDs immediately with telemetry?

An old Kafka record, replay, or delayed retry could arrive after the original
time-series row has expired. If its dedup ID had already disappeared, the old
event could be inserted again as though it were new.

The correct dedup horizon depends on:

- Kafka retention
- incident replay requirements
- raw telemetry retention
- compliance requirements
- maximum expected delayed-processing window

For now pruning is an explicit operator action rather than an invisible
background job.
