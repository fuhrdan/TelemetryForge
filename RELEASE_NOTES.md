# TelemetryForge v1.7.0 Development Release Notes

## Change Intelligence

v1.7.0 makes production changes first-class telemetry evidence and connects
change history to the existing Flight Recorder, Evidence Graph, portable
incident archive, and tenant-scoped query model.

### Structured change ingestion

Added:

```text
POST /api/v1/changes
```

for:

```text
deployment
rollback
release
feature_flag
configuration
infrastructure
```

Change identity is tenant-scoped and records source, environment, version,
previous version, Git SHA, build ID, actor, summary, and explicit rollback
relationships.

### Evidence-preserving worker placement

Change markers are normalized and stored before lossy shaping:

```text
Flight Recorder
 -> Normalizer
 -> Schema Intelligence
 -> Change Intelligence
 -> Adaptive Shaping
 -> Cardinality Firewall
 -> Persistence
 -> Routing
 -> Incident Detector
```

The v1.7 integration also corrects the actual worker ordering so Adaptive
Shaping now precedes Cardinality Firewall as intended by the v1.4 design.

### Before / after analysis

A change analysis compares bounded source telemetry before and after a selected
change using event count, error rate, and p95 latency.

Sources with insufficient traffic remain `insufficient` instead of receiving a
noisy verdict. Materiality thresholds are documented and visible in the output.

### Observed blast radius

The analysis reports the percentage of assessable observed sources that
materially regressed. This is explicitly not presented as a dependency graph or
causal blast-radius proof.

### Rollback / recovery evidence

Explicit `rollback_of` markers are correlated with the original change.
Post-rollback recovery signals are surfaced as evidence that contradicts
continued degradation, without claiming the rollback caused recovery.

### Evidence Graph / portable archive

Evidence Graph now supports:

- structured change near incident;
- structured change before error;
- explicit rollback relationships;
- recovery after rollback.

`.tfincident` format v1 now optionally includes `change/markers.json`. The manifest schema itself remains unchanged; the new member is covered by the existing integrity map so older strict format-v1 readers can ignore it.

### API / CLI / dashboard

Added:

```text
GET  /api/v1/changes
POST /api/v1/changes/{id}/analyze
GET  /api/v1/changes/{id}/analysis
GET  /api/v1/incidents/{id}/changes
```

CLI:

```bash
telemetryctl change list
telemetryctl change show --id ...
telemetryctl change analyze --id ...
```

Dashboard adds a Change Intelligence view with deployment history, Git/version
metadata, source-level before/after deltas, observed blast radius, and
rollback/recovery evidence.

### Demonstration

```bash
make demo-change
```

generates healthy baseline -> deployment -> multi-source regression -> rollback
-> recovery traffic.

### Storage

Migration `014_change_intelligence.sql` adds normalized markers and regenerable
analysis snapshots.

### Safety language

v1.7 preserves the product rule:

> Temporal association is evidence for investigation, not proof of root cause.
