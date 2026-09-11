# TelemetryForge v2.0.0 Release Notes

## Evidence-First Intelligent Telemetry Control Plane

v2.0.0 completes the roadmap from durable telemetry ingestion to cited incident synthesis. The release adds a deterministic investigator above the existing Evidence Graph, Incident Replay, Cost Simulator, Change Intelligence, lifecycle, connector, and operational-proof foundations.

## Evidence-First Investigator

Added findings that include exact supporting/contradicting graph-edge citations and cited node IDs. Results use explicit `insufficient_evidence`, `mixed_evidence`, and `evidence_supported` states. The default engine makes no external model call and does not manufacture a root cause.

## Incident comparison

Added reproducible weighted comparison across captured sources, event types, hypotheses, and structured change versions/Git SHAs. The response cites matching nodes from both incidents and explicitly states that overlap is not shared causality.

## Advisory policy recommendations

The first v2 recommendation evaluates an already-recorded shadow policy when Cost Simulation shows a material reduction in canonical bytes or sampled series. Recommendations cite the simulation and, when available, completed Incident Replay evidence. They never activate production configuration.

Every production-affecting recommendation requires human approval and points back to the v1.6 lifecycle: shadow -> evidence review -> approval -> activation.

## Product surfaces

Added:

```text
GET /api/v1/incidents/{id}/investigation
GET /api/v1/incidents/{id}/compare?other=...
GET /api/v1/investigations
```

CLI:

```bash
telemetryctl intelligence investigate --id INC-42
telemetryctl intelligence compare --left INC-42 --right INC-17
telemetryctl intelligence list
```

The dashboard now presents the Evidence-First Investigator, citations, contradictions, evidence counts, and human-approval warnings.

## Storage

Migration `017_intelligent_control_plane.sql` stores the latest tenant-scoped investigation snapshot per incident.

## Documentation / decisions

Added v2 intelligence guides and ADRs 0049-0051 covering deterministic cited synthesis, no autonomous production mutation, and the non-causal meaning of incident similarity.

## Compatibility and safety

The v2 layer is additive. Existing ingestion, Flight Recorder, Schema/Cardinality Intelligence, Adaptive Shaping, Router, `.tfincident`, lifecycle, connector and `.tfproof.json` contracts remain intact.

The investigator is not in the ingestion critical path and cannot mutate production policy.

## Validation note

Repository-local focused tests and artifact checks are run before packaging. The complete dependency-backed Go suite remains CI-authoritative where the required Go 1.27.1 toolchain and external dependencies are available.
