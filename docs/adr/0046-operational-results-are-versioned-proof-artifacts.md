# ADR 0046: Operational Results Are Versioned Proof Artifacts

**Status:** Accepted  
**Date:** 2026-09-11

## Context

Performance/HA claims become stale or misleading when detached from test
configuration and raw evidence.

## Decision

Represent reviewed operational runs as `.tfproof.json` with assertions,
measurements, evidence references, and configuration SHA-256 fingerprints.
Record the whole-file hash and byte count when importing into PostgreSQL.

## Consequences

Portfolio/release claims can point to reproducible evidence. The project does
not need hard-coded benchmark numbers in source documentation.
