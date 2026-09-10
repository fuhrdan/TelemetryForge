# Connector Operations

Show built-ins with `telemetryctl connector catalog`. Validate routing and connector definitions with `telemetryctl connector validate --file routing/active.json`. List recent runtime heartbeats with `telemetryctl connector list`.

Probe one configured destination without sending telemetry:

```bash
telemetryctl connector test --destination primary
```

Sending a synthetic event is explicitly opt-in with `--send-sample`; use `--metric` for a Prometheus-compatible numeric sample. Do not point sample delivery at production unless the side effect is intended.

An unhealthy connector while the router remains ready is expected: each destination is isolated.
