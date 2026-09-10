# ADR 0044: Preserve Legacy Routing Through Connector Translation

**Status:** Accepted  
**Date:** 2026-09-10

## Context
v1.3-v1.7 immutable routing artifacts use top-level `kafka` and `http` destinations.

## Decision
Continue accepting them and translate them internally into v1.8 connector specs. New documents should use `type: connector`.

## Consequences
Historical lifecycle and `.tfincident` routing snapshots remain parseable while runtime delivery has one connector implementation path.
