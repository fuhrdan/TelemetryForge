# eBPF edge collection

TelemetryForge v2.8.0 adds an **optional Linux eBPF collector** to `telemetryforge-edge`. The collector is deliberately small: tracepoint programs increment aggregate counters in kernel BPF array maps, and user space polls those counters into the existing durable edge path.

## v2.8 signals

| Signal | Linux tracepoint | TelemetryForge meaning |
| --- | --- | --- |
| `process_exec` | `sched/sched_process_exec` | process executions observed since the prior persisted sample |
| `socket_connect` | `syscalls/sys_enter_connect` | socket connect attempts observed since the prior persisted sample |
| `tcp_retransmit` | `tcp/tcp_retransmit_skb` | TCP retransmission events observed since the prior persisted sample |

The kernel program does **not** copy packet bodies, socket addresses, DNS names, command lines, environment variables, or process arguments. v2.8 is an aggregate counter collector, not a packet sniffer.

## Durability boundary

Collection does not bypass the edge WAL:

```text
Linux tracepoint
      |
      v
small eBPF counter program
      |
      v
BPF array map
      |
      v
poll counter delta
      |
      v
edge WAL + fsync
      |
      v
configured replication quorum
      |
      v
normal mesh/Kafka delivery
```

If local WAL persistence fails, the collector retains the unpersisted delta and retries it on a later poll. If the WAL persisted the metric but a later replication acknowledgement fails, the collector advances its local counter baseline because the WAL now owns retry/replay. This avoids manufacturing duplicate counter deltas during a temporary quorum outage.

## Enabling the collector

The collector defaults to disabled.

```bash
export TELEMETRYFORGE_EBPF_ENABLED=true
export TELEMETRYFORGE_EBPF_SIGNALS=process_exec,socket_connect,tcp_retransmit
export TELEMETRYFORGE_EBPF_POLL_INTERVAL=5s
export TELEMETRYFORGE_EBPF_TENANT_ID=default
```

Host-level kernel metrics are assigned to `TELEMETRYFORGE_EBPF_TENANT_ID` (the edge default tenant when unset). `TELEMETRYFORGE_EBPF_REQUIRED=false` is the default. In that mode an attach failure is visible through `/edge/status` and `/edge/ebpf/status`, but the edge remains available for normal ingest. Set it to `true` only when kernel telemetry is a deployment requirement; the edge then fails startup if the probes cannot attach.

## Linux permissions

Live attachment requires Linux tracefs plus permission for the BPF and perf-event syscalls. On modern kernels that normally means `CAP_BPF` and `CAP_PERFMON` (or an equivalent privileged policy), and the container seccomp profile must permit the syscalls. Cluster policy and kernel lockdown can still deny attachment.

The checked-in Kubernetes eBPF overlay intentionally requires an explicit opt-in because these capabilities increase the container's kernel-observation privileges. Do not apply the overlay to environments where kernel telemetry is not needed.

## Verification

Static/program construction check:

```bash
go run ./cmd/ebpfcheck
```

Target-node live verifier check:

```bash
telemetryforge-ebpfcheck --live
```

A live check is host-specific evidence. The normal CI proof does not claim successful privileged attachment on GitHub-hosted runners.

## Explicit non-goals

v2.8.0 does not claim packet capture, per-flow attribution, DNS inspection, syscall argument capture, universal kernel support, or zero-overhead collection. It also does not move routing, retention, autonomous control, or policy evaluation into eBPF.
