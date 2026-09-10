# ADR 0034: Preserve Evidence Before Shaping

**Status:** Accepted  
**Date:** 2026-09-09

## Context

Sampling and attribute removal are irreversible if performed before incident
evidence is captured.

## Decision

The worker order is:

```text
Flight Recorder -> normalize -> schema -> cardinality -> shaping -> persistence/routing
```

A sampled-out event is a successful terminal processing decision, not a failure.

The shaping decision must be durably accounted for before an active sample-out
or transform is applied. A compact `(tenant,event,config,version)` decision
ledger makes the first pressure-aware decision stable across whole-chain retries
and increments minute aggregates only once. If shaping-audit persistence fails,
the worker keeps the original event instead.

## Consequences

The Flight Recorder remains a full-fidelity source for incident review and
shaping preview.

Sampling cannot silently create a DLQ storm, downstream retries cannot silently
change an already-recorded sampling decision, and audit outages bias toward more
telemetry rather than unexplained loss.
