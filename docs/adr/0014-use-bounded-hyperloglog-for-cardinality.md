# ADR 0014: Use Bounded HyperLogLog for Cardinality Warnings

**Status:** Accepted  
**Date:** 2026-09-09

## Context

The Cardinality Firewall needs to detect growing unique tag values without
keeping every raw identifier in memory.

An exact set per source/type/tag can consume memory proportional to traffic and
would itself become vulnerable to the cardinality problem it is meant to detect.

## Decision

Use a fixed 16-hash exact window followed by a small 64-register
HyperLogLog estimator for each tracked source/type/dimension combination.

Also:

- cap the global number of tracked dimensions;
- evict least-recently-seen dimension state at the cap;
- use SHA-256-derived fingerprints rather than storing raw tag values;
- require a minimum observation window before generic growth projection; and
- rate-limit repeated operational findings.

## Alternatives considered

### Exact hash sets

Easy to reason about, but memory grows with unique values.

### External Redis cardinality state

Useful for distributed global estimates later, but adds a new mandatory service
before Kubernetes replica-local policy behavior is proven.

### Full vendor/backend cardinality API

Would couple the control plane to one observability vendor.

## Consequences

Estimates are approximate and replica-local in v0.8.0. A highly distributed
worker fleet can therefore under-estimate global cardinality compared with a
shared estimator.

The feature remains valuable as an early firewall, but a later scale milestone
may move estimator state to a shared mergeable representation.
