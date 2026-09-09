# ADR 0017: Isolate Incident Replay from Production Flow

**Status:** Accepted  
**Date:** 2026-09-09

## Context

Frozen incident telemetry is valuable for testing new processing/policy behavior,
but replaying it through the normal ingest path can create duplicate production
telemetry, alerts, incidents, or vendor costs.

## Decision

Incident Replay is analysis-only by default.

It executes:

- normalization
- active policy
- optional shadow policy
- compact replay-result persistence

It does not execute normal primary telemetry persistence, Flight Recorder
capture, automatic incident detection, or Kafka publication.

Optional publication is allowed only to a dedicated `telemetry.replay`
namespace and requires an explicit operator flag.

The safety restriction exists in the replay library as well as the CLI so a
future caller cannot bypass it accidentally.

## Consequences

Replay results are intentionally not a byte-for-byte clone of the production
worker chain. They test policy/normalization effects safely.

A later isolated replay environment can add more processors, but production
side effects remain opt-in and namespaced.
