# Visibility Preview

Sampling should be reviewed against real evidence before promotion.

TelemetryForge v1.4.0 can evaluate a shaping configuration against a frozen
incident without writing production shaping/cardinality state.

## CLI

```bash
telemetryctl shaping preview \
  --incident INC-42 \
  --file shaping/active.json \
  --candidate shaping/shadow.json \
  --policy policies/active.json \
  --pressure 0.90
```

The active Cardinality Firewall policy is replayed first using isolated local
cardinality state, matching production ordering without contaminating shared
production counters. Shaping is then previewed against that post-policy event.

The result reports active and candidate measurements such as:

- observed events;
- kept events;
- sampled-out events;
- protected events;
- transformed events;
- dropped oversized payloads;
- canonical bytes before/after shaping;
- event retention percentage;
- byte retention percentage;
- protected-event retention percentage; and
- source/type coverage percentage.

## Why there is no single visibility score

TelemetryForge deliberately does not collapse these measurements into one
opaque "visibility score."

Keeping 20% of healthy request metrics while preserving 100% of incident/error
signals is a different operational choice from keeping 20% of everything.

The preview therefore exposes the measurable components so an operator can make
the tradeoff explicitly.

## Reproducibility

Sampling uses the event ID plus shaping config name/version. For a fixed pressure
value the same frozen incident produces repeatable decisions.

## Limits

The preview measures the frozen incident sample. It cannot guarantee what a
future production distribution will look like.

Use it together with:

- Shadow Shaping on live traffic;
- Telemetry Cost Simulator;
- Incident Replay; and
- the Evidence Graph.
