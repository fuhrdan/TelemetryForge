# ADR 0036: Use ZIP, JSON/JSONL, and an Integrity Manifest for `.tfincident`

**Status:** Accepted  
**Date:** 2026-09-09

## Context

Portable incidents should remain inspectable without proprietary tooling while
preserving exact evidence/configuration context.

## Decision

The unencrypted `.tfincident` format is ordinary ZIP.

Use:

- JSON for metadata/analysis/configuration;
- JSONL for frozen event records;
- `manifest.json` for format identity and per-member SHA-256/byte length.

The reader rejects extra, missing, modified, duplicate, and unsafe-path members.

## Alternatives

### Custom binary format

Rejected because it makes inspection/debugging unnecessarily dependent on
TelemetryForge.

### Tar archive

Viable, but ZIP has strong cross-platform tooling and direct random member
access.

### One giant JSON file

Rejected because JSONL is friendlier for large event sequences and partial
inspection.

## Consequences

Unencrypted archives are easy to inspect.

ZIP parsing must enforce explicit size/member bounds to avoid archive-bomb/path
traversal risks.
