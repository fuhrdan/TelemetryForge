# ADR 0056: Formal models prove bounded safety, not environmental liveness

**Status:** Accepted for v2.5.0.

## Context

TelemetryForge now has multiple durability boundaries: local WAL acceptance, replicated quorum, global route failover, and signed cryptographic lineage. Unit and chaos tests can demonstrate observed behavior, but they do not make the allowed protocol state transitions explicit enough to exhaustively inspect small state spaces.

At the same time, a broad statement such as “formal verification proves zero loss” would overstate what a finite model establishes. Cloud recovery, hardware survival, scheduler fairness, and eventual downstream availability are environmental assumptions rather than safety properties of the protocol alone.

## Decision

Maintain TLA+ specifications for the safety-critical protocol abstractions and run TLC in CI over finite model configurations.

Maintain a second dependency-free Go state-space checker that mirrors the release invariants. Use it for ordinary Go CI, fuzzed schedule testing, and operational proof evidence.

The v2.5 formal claim is limited to the documented safety invariants and explored model bounds. Liveness and real-system behavior continue to require integration and operational evidence.

## Consequences

Positive:

- acknowledgement and deletion rules are reviewable independently of implementation code;
- small interleaving/state-space errors can fail CI before deployment;
- formal results are reproducible and evidence-backed;
- the project can discuss proof scope without claiming impossible guarantees.

Tradeoffs:

- models can diverge from implementation if not maintained;
- finite state exploration is not a proof of an unbounded environment;
- TLC adds a Java/tooling dependency to one CI job;
- formal modeling does not replace race, integration, chaos, or recovery testing.
