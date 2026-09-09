# Retention

Telemetry volume grows continuously. Retention is therefore part of the data
model, not a cleanup task to think about later.

## Current development default

The local TimescaleDB migration installs a **30-day retention policy** for
`telemetry_events`.

```sql
SELECT add_retention_policy(
    'telemetry_events',
    INTERVAL '30 days',
    if_not_exists => TRUE
);
```

The value is intentionally visible in SQL so operators can review the behavior
without searching application code.

## Why 30 days?

It is a development default, not a universal production recommendation. It is
long enough to make the local system useful while demonstrating that telemetry
has an explicit lifecycle.

A production deployment should base retention on:

- incident investigation requirements
- compliance requirements
- telemetry value
- storage cost
- source/type
- whether lower-resolution aggregates can replace raw events

Later TelemetryForge releases will connect retention and sampling choices to
the Telemetry Cost Simulator.
