# ADR 0033: Deterministic Pressure-Aware Sampling

**Status:** Accepted  
**Date:** 2026-09-09

## Context

Random per-process sampling is difficult to reproduce in incident replay and can
make two workers decide differently for the same retried event.

High queue pressure can also make a fixed healthy-traffic rate unnecessarily
expensive during bursts.

## Decision

Sampling is deterministic from shaping config identity plus canonical event ID.

Worker queue utilization can reduce the configured base rate at reviewed high
and critical watermarks, while each rule retains an explicit minimum floor.

Protected error/incident/high-latency classes always keep the event.

## Consequences

Retries/replays are stable for a fixed config and pressure input.

Pressure adaptation is intentionally local to a worker queue and is not claimed
as cluster-global load control.
