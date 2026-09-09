# Automatic Incident Capture

TelemetryForge can automatically freeze the rolling Incident Flight Recorder when
simple operational thresholds are crossed.

## Current rules

### High latency

A metric is considered for latency detection when:

- it has a numeric value;
- its unit is `ms`; and
- its type contains `latency` or `duration`.

Default trigger:

```text
value >= 1000 ms
```

### Error burst

An event counts as an error when:

- its type contains `error`; or
- tag `severity` / `level` is `error`, `critical`, or `fatal`.

Default trigger:

```text
5 errors from one source within 1 minute
```

## Freeze window

Automatic incidents preserve the previous 15 minutes of Flight Recorder data.

The trigger occurs after the telemetry event has successfully reached the main
database. This avoids creating incidents for events that never became durable.

## Cooldown

The same source/reason category has a default 10-minute cooldown. Without a
cooldown, every high-latency sample during one outage could create another
incident.

## Configuration

```text
TELEMETRYFORGE_INCIDENT_LATENCY_MS=1000
TELEMETRYFORGE_INCIDENT_ERROR_COUNT=5
```

The error window, cooldown, and lookback are currently code-level defaults.
They become policy-as-code in a later release.

## Failure behavior

If automatic incident freezing fails after the primary event is stored,
TelemetryForge logs that incident-capture failure but does not retry or DLQ the
already-persisted telemetry event.

That distinction prevents an auxiliary incident feature from corrupting normal
telemetry processing semantics.


## Bounded detector state

Source names are telemetry input and may themselves have high cardinality.

The in-memory automatic-incident detector therefore caps tracked source state at
10,000 sources by default. If the cap is reached, the oldest error-window or
cooldown state is evicted rather than allowing arbitrary source names to grow
memory without bound.

This is separate from the v0.7.0 Cardinality Firewall, which protects metric/tag
dimensions before downstream forwarding.

## Failed freeze behavior

A cooldown reservation is created before storage I/O to suppress concurrent
duplicate freezes from the same source/reason category.

If the freeze itself fails, that reservation is released immediately so the
next qualifying event can retry capture. If the incident was frozen but only
its annotation fails, the cooldown remains because useful evidence already
exists.
