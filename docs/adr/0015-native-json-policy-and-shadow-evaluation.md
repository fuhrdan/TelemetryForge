# ADR 0015: Native JSON Policy and Shadow Evaluation

**Status:** Accepted  
**Date:** 2026-09-09

## Context

Telemetry policy changes can reduce cost but also remove useful evidence. The
project needs policy-as-code plus a safe way to evaluate proposed rules.

## Decision

Use strictly parsed, versioned JSON policy files loaded by the Go worker.

Evaluate:

- one **active** policy that may change the normal event representation; and
- one optional **shadow** policy that records only decision differences.

Policy files are validated at startup and with `telemetryctl policy validate`.

## Alternatives considered

### Open Policy Agent / Rego

Powerful and mature, but adds a runtime/tooling dependency before the current
rules require arbitrary policy language.

### Hard-coded Go thresholds

Simple but turns operational policy changes into application releases.

### YAML

Human-friendly, but would require another Go parser dependency. JSON remains
easy to review and generate.

## Consequences

The v0.8 policy model is intentionally limited to Cardinality Firewall
decisions. If future policy domains outgrow this model, TelemetryForge can add a
versioned policy backend or OPA integration while keeping shadow evaluation as a
core safety concept.
