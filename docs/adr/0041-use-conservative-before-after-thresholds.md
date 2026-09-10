# ADR 0041: Use Conservative Before/After Thresholds

**Status:** Accepted  
**Date:** 2026-09-10

## Context

Small traffic samples and ordinary variance can make arbitrary before/after
comparisons look dramatic.

## Decision

Require at least five events in both windows before classifying a source.

A regression needs either:

- >= 5 percentage-point error-rate increase and at least two additional errors;
  or
- >= 100 ms p95 increase and >= 25% relative increase.

Expose raw before/after values and the threshold basis.

## Consequences

The system will intentionally return `insufficient` or `unchanged` in marginal
cases rather than manufacture a high-confidence change verdict.
