# ADR 0037: Use Explicit-Key AES-256-GCM for Optional Archive Encryption

**Status:** Accepted  
**Date:** 2026-09-09

## Context

Incident archives can contain sensitive full-fidelity telemetry.

Password encryption would require choosing and versioning a KDF policy.

## Decision

Optionally wrap the complete `.tfincident` ZIP in AES-256-GCM.

Require an explicit random 32-byte key represented as 64 hexadecimal
characters.

Do not accept passwords in v1.5.

## Consequences

Wrong keys and modified ciphertext fail authenticated decryption.

Key generation/distribution remains an operator/secret-manager responsibility.

A future archive version can add recipient/public-key encryption without
weakening the v1 envelope.
