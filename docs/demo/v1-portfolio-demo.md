# v1.0.0 Portfolio Demo

This scenario demonstrates the product story rather than clicking randomly
through every endpoint.

## 1. Start the local stack

```bash
docker compose up --build
```

Open:

```text
TelemetryForge: http://localhost:3000
Grafana:        http://localhost:3001
```

## 2. Generate evidence-shaped traffic

```bash
make demo-evidence
```

The scenario emits:

1. a deployment marker;
2. high checkout latency;
3. errors sharing trace/correlation evidence; and
4. a later recovery signal.

The automatic incident detector should freeze a window.

## 3. Inspect the incident

Select the incident in the dashboard.

Show:

- frozen Flight Recorder timeline;
- Evidence Graph disclaimer;
- supporting change/error temporal association;
- latency-before-error relationship;
- contradictory recovery evidence.

The key message is:

> TelemetryForge shows what the evidence supports and contradicts without
> claiming that temporal association proves root cause.

## 4. Replay policy

```bash
go run ./cmd/telemetryctl incident replay \
  --id <INCIDENT_ID>
```

Refresh the dashboard and show replay effects.

## 5. Simulate telemetry cost

```bash
go run ./cmd/telemetryctl cost simulate \
  --incident <INCIDENT_ID>
```

Explain that dollar output is intentionally absent without reviewed pricing.

## 6. Show self-observability

Open Grafana and show:

- gateway rate/latency;
- worker queue;
- real Kafka group lag;
- retry/DLQ counters;
- traces through gateway -> Kafka -> worker.

## 7. Explain production hardening

Point to:

```text
deployments/kubernetes/overlays/production/
```

and summarize:

- scoped authentication
- tenant isolation
- server-side dashboard credential
- response redaction
- Kafka TLS/SCRAM
- non-root/seccomp
- authenticated metrics

This demonstrates engineering judgment without pretending the local demo
credentials are production credentials.
