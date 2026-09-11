# Evidence-First Investigator

TelemetryForge v2.0.0 turns the existing Evidence Graph into a deterministic incident synthesis layer.

The investigator is deliberately **not** a root-cause oracle. It does not call an external language model, invent missing observations, or replace operator judgment. It summarizes the graph relationships already captured by TelemetryForge and returns exact citations to the node IDs and graph edges that support or contradict each finding.

## Status values

```text
insufficient_evidence
mixed_evidence
evidence_supported
```

`insufficient_evidence` is a successful result. It means the captured data does not support a stronger built-in hypothesis.

`mixed_evidence` means at least one lead has both supporting and contradictory graph relationships.

`evidence_supported` means one or more built-in hypotheses have captured supporting evidence without contradiction. It still does **not** mean TelemetryForge has proved root cause.

## Citations

Each finding can include:

- supporting Evidence Graph edges;
- contradicting Evidence Graph edges;
- the exact node IDs referenced by those edges;
- the evidence reason recorded on each edge.

The built-in synthesis currently understands the graph hypotheses created for recent changes, latency preceding errors, and sustained degradation/recovery. Unknown hypotheses are not silently generalized.

## Bounded behavior

The investigator operates on the already bounded incident graph and stored replay/cost history. It does not perform open-ended searches or background work.

## API

```text
GET /api/v1/incidents/{id}/investigation
GET /api/v1/investigations
```

The incident route regenerates the Evidence Graph from the frozen incident, nearby structured changes, replay history, and cost simulation history, then stores the latest deterministic investigation snapshot.

## CLI

```bash
telemetryctl intelligence investigate --id INC-42
telemetryctl intelligence list
```

See [Policy Recommendations](policy-recommendations.md) for the production-change safety boundary.
