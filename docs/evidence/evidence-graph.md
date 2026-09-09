# Evidence Graph

The Evidence Graph turns a frozen incident into an explainable relationship map.

It is **not** an automated root-cause engine.

Every graph response includes this invariant:

> Relationships are evidence associations, not automated root-cause claims.

## Inputs

The graph uses only captured TelemetryForge evidence:

- frozen incident events
- canonical correlation IDs
- captured `trace_id` tags
- source/time ordering
- deployment/change markers
- latency/error/recovery signals
- replay history for the incident
- cost simulations for the incident

It does not invent infrastructure dependencies that were never observed.

## Edge assessments

### `supporting`

A captured relationship adds evidence for an investigation hypothesis.

Examples:

- two events share a correlation ID;
- two events share a trace ID;
- high latency precedes an error on the same source;
- a captured deployment marker precedes an error.

Supporting evidence is not proof of causation.

### `contradicting`

A captured relationship weakens a hypothesis.

Example:

- normal latency after an error contradicts a hypothesis that degradation
  remained continuously sustained.

### `related`

Useful context with no stronger evidentiary claim.

Example:

- two events occurred on the same source within 30 seconds.

## Built-in hypotheses

The current graph can summarize:

- recent change associated with errors;
- latency appearing before errors;
- sustained degradation versus later recovery.

If no built-in hypothesis is supported, the graph returns an
`insufficient-evidence` hypothesis instead of fabricating a conclusion.

## Bounds

The API reads at most 1,000 frozen incident events.

Graph construction sorts event groups and creates a bounded number of
relationship edges. It does not perform an unrestricted all-pairs comparison.

## API

```text
GET /api/v1/incidents/{id}/evidence-graph
```

The API:

1. loads the incident inside the authenticated tenant;
2. loads that tenant's replay/cost history;
3. regenerates the graph;
4. stores the latest graph snapshot; and
5. returns the graph.

## CLI

```bash
telemetryctl incident graph \
  --id INC-42 \
  --tenant production
```

## Stored snapshots

`evidence_graph_snapshots` stores the latest graph JSON for each
`tenant_id / incident_id`.

The frozen incident remains the source of truth. A graph snapshot is a
regenerable analysis artifact.
