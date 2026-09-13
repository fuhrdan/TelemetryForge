#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/_common.sh"
require_execute "$@"
started=$(iso_now)
stamp=$(date +%s)
log="$RESULTS/raw/fastpath-performance-$stamp.log"
report="$RESULTS/raw/fastpath-benchmark-$stamp.json"
out="$RESULTS/fastpath-performance-$stamp.tfproof.json"

set +e
{
  echo "== Fast-path correctness =="
  go test -count=1 ./internal/fastpath ./internal/wal ./internal/edge
  echo "== Fast-path microbenchmarks =="
  go test -run '^$' -bench 'Benchmark(Ring|BytePool|JSON)' -benchmem -count=5 ./internal/fastpath
} >"$log" 2>&1
rc=$?
set -e

measurement_args=()
if [[ $rc -eq 0 ]]; then
  python3 scripts/parse-fastpath-bench.py "$log" --out "$report"
  while IFS= read -r measurement; do
    measurement_args+=(--measurement "$measurement")
  done < <(python3 - "$report" <<'PYMEASURE'
import json, sys
report=json.load(open(sys.argv[1]))
for bench in report.get("benchmarks", []):
    name=bench["name"].replace(":", "_")
    print(f"{name}_median_ns:{bench['median_ns_per_op']}:ns/op")
    print(f"{name}_median_bytes:{bench['median_bytes_per_op']}:B/op")
    print(f"{name}_median_allocs:{bench['median_allocs_per_op']}:allocs/op")
PYMEASURE
  )
else
  printf '{"format":"telemetryforge-fastpath-benchmark","format_version":1,"error":"benchmark execution failed; inspect raw log"}\n' > "$report"
fi
status=$([[ $rc -eq 0 ]] && echo pass || echo fail)

write_proof --out "$out" --scenario fastpath-performance --status "$status" --started-at "$started" \
  --assertion "durability-boundary:$status:fast-path primitives do not replace WAL fsync replication quorum or ordered checkpointing" \
  --assertion "bounded-ring:$status:ring refuses overwrite when full and passes concurrent producer-consumer tests" \
  --assertion "pooled-buffers:$status:bounded buffer pools are exercised without retaining oversized payload buffers" \
  --assertion "benchmark-evidence:$status:Go benchmark output with allocation metrics was captured without converting it into a universal capacity claim" \
  --evidence "benchmark-log:$log" \
  --evidence "benchmark-report:$report" \
  --config "fastpath-pool:$ROOT/internal/fastpath/pool.go" \
  --config "fastpath-ring:$ROOT/internal/fastpath/ring.go" \
  --config "edge-replay:$ROOT/internal/edge/publisher.go" \
  --config "kafka-publisher:$ROOT/internal/stream/kafka.go" \
  "${measurement_args[@]}" \
  --note "Microbenchmarks describe this runner only; they are not end-to-end throughput or production capacity claims."

echo "$out"
exit "$rc"
