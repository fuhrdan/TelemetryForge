#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/_common.sh"
require_execute "$@"
started=$(iso_now)
stamp=$(date +%s)
log="$RESULTS/raw/ebpf-edge-$stamp.log"
report="$RESULTS/raw/ebpf-check-$stamp.json"
out="$RESULTS/ebpf-edge-$stamp.tfproof.json"

set +e
{
  go test -race -count=1 ./internal/ebpf
  go run ./cmd/ebpfcheck --output "$report"
} >"$log" 2>&1
rc=$?
set -e
status=$([[ $rc -eq 0 ]] && echo pass || echo fail)

write_proof --out "$out" --scenario ebpf-edge --status "$status" --started-at "$started" \
  --assertion "bounded-kernel-program:$status:dependency-free tracepoint program template uses only a one-entry counter map and atomic increment" \
  --assertion "durable-handoff:$status:collector tests retain deltas until local WAL persistence and avoid re-emission after local persistence" \
  --assertion "aggregate-privacy-boundary:$status:v2.8 emits aggregate counts only and does not copy packet payloads process arguments or DNS names" \
  --assertion "optional-degradation:$status:collector tests confirm non-required attach failures leave the edge available while required mode fails closed" \
  --evidence "log:$log" \
  --evidence "ebpf-report:$report" \
  --config "collector:$ROOT/internal/ebpf/model.go" \
  --config "kernel-loader:$ROOT/internal/ebpf/syscall_linux.go" \
  --config "edge-integration:$ROOT/cmd/edge/main.go" \
  --note "This proof validates program construction, collector semantics, and the durable handoff without requiring privileged BPF attachment. Live attach depends on host tracefs, kernel policy, seccomp, and BPF/perf capabilities and must be verified on the target Linux node with telemetryforge-ebpfcheck --live."

echo "$out"
exit "$rc"
