# TelemetryForge v2.8.0 Release Notes

## eBPF Edge Collection

TelemetryForge v2.8.0 adds optional Linux kernel-level collection while preserving every durable-ingest, replication, routing, lineage, formal-verification, fast-path, and autonomous-control boundary delivered in v2.1-v2.7.

### Dependency-free tracepoint counters

The edge can attach tiny eBPF tracepoint programs for `sched_process_exec`, `sys_enter_connect`, and `tcp_retransmit_skb`. Each program performs only an atomic increment in a one-entry BPF array map. User space polls absolute counters and converts only newly observed counts into canonical TelemetryForge metrics.

No packet payload, socket address, DNS name, command line, process argument, environment, or arbitrary syscall data is copied by the v2.8 kernel program.

### Durable collector handoff

Kernel metrics use the existing edge publisher. The new publish receipt distinguishes local WAL fsync from later replication acknowledgement. If a write never reaches the local WAL, its counter delta remains pending and is retried. Once the WAL persists it, the collector advances its baseline even if quorum acknowledgement later times out, because normal WAL replay owns recovery from that point.

### Deployment safety

eBPF remains disabled by default. Optional mode reports attach failure but leaves ordinary edge ingest available. `TELEMETRYFORGE_EBPF_REQUIRED=true` makes the configured collector a startup requirement.

The repository includes an explicit Kubernetes overlay and Docker Compose override for Linux hosts that provide tracefs plus BPF/perf permissions. The ordinary base deployment remains unprivileged.

### Verification

`telemetryforge-ebpfcheck` validates the dependency-free program template and host surfaces. `--live` asks the target kernel to verify/attach the programs and read a snapshot; this is intentionally host-specific evidence rather than a CI portability claim.

`proof/ebpf-edge.sh` exercises the race-tested collector semantics and emits `.tfproof.json` evidence without pretending a hosted CI runner proves production-kernel attachment.

### Explicit non-claims

v2.8 does not claim packet capture, per-flow attribution, DNS inspection, universal Linux/kernel/container support, zero overhead, or kernel-resident routing/policy/autonomous control.
