# Change Intelligence

TelemetryForge v1.7.0 makes deployments, rollbacks, releases, feature-flag
changes, configuration changes, and infrastructure changes first-class
incident evidence.

The feature is intentionally **evidence-first**. A change occurring before a
failure is useful investigation evidence, but temporal proximity alone does not
prove root cause.

## Structured marker

A normalized change marker records:

- tenant and `change_id`;
- canonical event ID;
- source/service;
- change kind and status;
- environment;
- version / previous version;
- Git SHA;
- build ID;
- actor;
- explicit `rollback_of` relationship;
- human-readable summary;
- change timestamp.

The marker is written by the worker after Flight Recorder + normalization +
Schema Intelligence, but **before Adaptive Shaping**. Operational changes are
also protected by the checked-in shaping policy.

## Before/after comparison

The default analysis compares 15 minutes before and 15 minutes after the
change. Each observed source receives:

- event count;
- error count / error rate;
- p95 latency for millisecond latency/duration metrics;
- delta and assessment.

A source needs at least five events on both sides before TelemetryForge labels
it `regressed`, `improved`, or `unchanged`. Otherwise it remains
`insufficient`.

Material regression requires either:

- error-rate increase of at least 5 percentage points plus at least two
  additional errors; or
- p95 latency increase of at least 100 ms and at least 25%.

These are conservative product defaults, not statistical proof.

## Blast radius

`blast_radius_percent` means:

```text
regressed assessable sources / all assessable observed sources
```

It is not a topology/dependency graph and does not claim every affected source
was caused by the selected change.

## Rollback evidence

TelemetryForge looks up a rollback within two hours when it either:

- explicitly references `rollback_of=<change_id>`; or
- affects the same source.

A normal post-rollback signal can be recorded as recovery evidence. The UI and
Evidence Graph explicitly state that recovery after rollback is association,
not proof that rollback caused recovery.

## Evidence Graph

Structured changes enrich an incident graph even when the deployment happened
just outside the frozen event window. The graph can add:

- `change-near-incident`;
- `structured-change-precedes-error`;
- `explicit-rollback-of`;
- `recovery-after-rollback`.

See [Evidence Graph](../evidence/evidence-graph.md).

## Portable incidents

`.tfincident` format v1 now optionally includes:

```text
change/markers.json
```

as an optional `change/markers.json` member listed in the manifest integrity map.

Older v1 readers can ignore the optional member; current readers verify its
integrity and count like all other archive evidence.
