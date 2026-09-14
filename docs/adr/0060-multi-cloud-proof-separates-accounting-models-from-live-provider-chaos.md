# ADR 0060: Multi-cloud proof separates accounting models from live provider chaos

**Status:** Accepted for v2.9.0.

TelemetryForge must be able to make precise durability claims without pretending a hosted CI runner can reproduce a public-cloud regional outage. v2.9 therefore separates **protocol/accounting proof** from **environmental fault injection**.

The checked-in multi-cloud proof model uses AWS, GCP, Azure, and bare-metal failure domains and exercises cloud loss, regional partition, capacity exhaustion, ambiguous delivery acknowledgement, and cascading cloud loss. Every accepted event is explicitly classified as uniquely delivered, durably retained, or lost; a passing proof requires zero lost, zero corrupted, and zero unaccounted accepted events. When durability capacity falls below quorum, new work must be rejected before acceptance rather than silently discarded.

The model is deterministic, dependency-free, race-tested, and emits machine-readable evidence. A local Docker Compose override provides the same logical failure-domain labels for integration exercises, but those containers are still one host and are not evidence of independent cloud infrastructure.

Real-provider chaos is a separate operator activity. Results from AWS/GCP/Azure fault injection may be published only when the evidence identifies the actual environment, scenario, configuration fingerprints, and observed accounting results. Provider outages, WAN latency, and recovery-time claims must never be inferred from the deterministic model alone.
