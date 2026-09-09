# ADR 0012: Start Automatic Incident Capture with Explicit Thresholds

**Status:** Accepted  
**Date:** 2026-09-09

## Context

The Flight Recorder becomes substantially more useful if likely incidents are
preserved automatically rather than relying on an operator to notice a problem
before the rolling buffer expires.

## Decision

v0.6.0 uses two deliberately understandable rules:

- millisecond latency/duration metrics at or above a configured threshold;
- an error-count threshold per source inside a short time window.

A cooldown suppresses duplicate incident creation.

## Alternatives considered

- **AI/anomaly detection immediately:** more impressive-sounding but difficult
  to validate and explain without a baseline dataset.
- **No automatic capture:** safer but leaves the signature feature dependent on
  perfect operator timing.
- **Complex rule DSL now:** valuable later, but policy-as-code is already a
  planned milestone.

## Consequences

Automatic capture is deterministic, testable, and easy to explain. It will
produce false positives in some workloads until source-specific policy arrives.
The dashboard exposes the trigger reason so operators can understand why an
incident exists.
