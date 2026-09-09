# Telemetry Cost Simulator

The Telemetry Cost Simulator projects how active or candidate telemetry policy
changes the volume and series shape of a frozen incident sample.

It deliberately separates **measurable telemetry effects** from
**operator-supplied pricing assumptions**.

## Run without pricing

```bash
telemetryctl cost simulate \
  --incident INC-2026-0042 \
  --policy policies/active.json \
  --shadow-policy policies/shadow.json
```

This reports:

- canonical JSON bytes in the frozen sample
- exact distinct series in the frozen sample
- active-policy bytes / series
- candidate-policy bytes / series
- changed-event counts
- linear 30-day volume projection

No dollar amount appears.

## Run with pricing

Create a pricing model:

```json
{
  "name": "example-vendor",
  "currency": "USD",
  "ingest_per_gb": 0.0,
  "active_series_per_1000_month": 0.0,
  "notes": "Replace zeros with reviewed contract/public pricing inputs."
}
```

Then:

```bash
telemetryctl cost simulate \
  --incident INC-2026-0042 \
  --pricing pricing/my-reviewed-model.json
```

Dollar estimates are emitted **only** when non-zero pricing inputs are supplied.

## Projection model

### Volume

Observed canonical JSON bytes are scaled linearly from the incident event-time
window to a 30-day month:

```text
projected bytes =
    sample bytes × seconds in 30 days / sample window seconds
```

This is directional. Actual backend payload encoding, compression, aggregation,
retention, and vendor accounting can differ.

### Series

Series identity is:

```text
source + event type + sorted non-TelemetryForge tags
```

The simulator counts exact distinct series in the frozen sample.

It does not extrapolate series linearly because stable label combinations can
repeat indefinitely without creating new series.

When a pricing model charges per active series, the observed sample series count
is treated as the representative active-series input. That assumption is
included in every stored simulation result.

## Why frozen incidents?

A frozen incident is a known evidence window and is already protected from
rolling Flight Recorder expiry. It gives simulations a stable, reproducible
input set.

The simulator is not a replacement for vendor billing exports or a capacity
model. It is a pre-change comparison tool.

## API / dashboard

History:

```text
GET /api/v1/cost-simulations
```

The dashboard displays projected monthly volume, sample series reduction, and
pricing-model output when available.
