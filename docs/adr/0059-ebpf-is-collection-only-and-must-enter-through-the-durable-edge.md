# ADR 0059: eBPF is collection-only and must enter through the durable edge

**Status:** Accepted for v2.8.0.

TelemetryForge can benefit from kernel-level signals, but pushing routing, sampling policy, retention, or autonomous control into BPF would create a second policy engine with a much harder upgrade and verification surface.

v2.8 therefore restricts eBPF to **bounded collection**. Tiny tracepoint programs increment aggregate counters in one-entry BPF array maps. User space polls the maps, creates canonical TelemetryForge metric envelopes, and submits them through the existing durable edge publisher.

The collector distinguishes local WAL persistence from later replication acknowledgement. A delta is retried only when it did not reach the local WAL. Once local fsync succeeds, the WAL owns replay even if quorum acknowledgement returns an error. This preserves the v2.1-v2.7 durability semantics without generating a second copy of the same kernel delta.

The initial probes intentionally expose counts only. Packet payloads, DNS names, command lines, process arguments, and environment variables are outside the v2.8 kernel-to-user-space contract. eBPF remains disabled by default and host capabilities must be enabled explicitly.
