# ADR 0025: Observe Schema Before Policy and Fail Open

**Status:** Accepted  
**Date:** 2026-09-09

## Context

Schema Intelligence needs to see the application-produced shape even when a
Cardinality Firewall rule later removes an attribute.

At the same time, a registry database problem must not turn an advisory feature
into a telemetry outage.

## Decision

Run schema inspection after normalization and before active/shadow policy.

Keep the Flight Recorder before both stages.

Schema registration is fail-open by default. Registry failures are logged and
normal telemetry continues.

Operators may explicitly configure fail-closed behavior.

## Consequences

The registry can explain fields that policy later removes.

Schema history can have gaps during a registry outage, but durable telemetry
processing remains available by default.

Worker retry cannot double-count a successful observation because registry
observations are idempotent by tenant/event ID.
