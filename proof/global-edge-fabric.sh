#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/_common.sh"
require_execute "$@"
started=$(iso_now)
stamp=$(date +%s)
log="$RESULTS/raw/global-edge-fabric-$stamp.log"
report="$RESULTS/raw/fabric-contract-$stamp.json"
out="$RESULTS/global-edge-fabric-$stamp.tfproof.json"

set +e
{
  go test -race -count=1 ./internal/fabric
  go run ./cmd/fabriccheck --output "$report"
  python3 - "$report" <<'PY'
import json, sys
r=json.load(open(sys.argv[1], encoding='utf-8'))
assert r['passed'] is True
assert r['release'] == '3.0.0'
by={s['name']:s for s in r['scenarios']}
assert by['healthy-cross-cloud']['snapshot']['state'] == 'ready'
assert by['downstream-partition']['snapshot']['state'] == 'degraded'
assert by['downstream-partition']['snapshot']['acceptance_ready'] is True
assert by['quorum-loss']['snapshot']['acceptance_ready'] is False
assert by['wal-capacity']['snapshot']['acceptance_ready'] is False
assert by['required-ebpf-unavailable']['snapshot']['acceptance_ready'] is False
print('validated v3 global edge fabric contract')
PY
} >"$log" 2>&1
rc=$?
set -e
status=$([[ $rc -eq 0 ]] && echo pass || echo fail)

write_proof --out "$out" --scenario global-edge-fabric --status "$status" --started-at "$started" \
  --assertion "healthy-composition:$status:healthy cross-cloud durability routing lineage and fast-path state compose to ready" \
  --assertion "delivery-separation:$status:downstream partition degrades delivery while preserving safe WAL/quorum acceptance" \
  --assertion "quorum-fail-closed:$status:required durability quorum loss disables successful acceptance" \
  --assertion "capacity-fail-closed:$status:full durable WAL disables successful acceptance before overwrite" \
  --assertion "required-collector-fail-closed:$status:required eBPF collection cannot be reported healthy while inactive" \
  --evidence "log:$log" \
  --evidence "fabric-contract-report:$report" \
  --config "fabric-contract:$ROOT/internal/fabric/model.go" \
  --config "edge-integration:$ROOT/cmd/edge/main.go" \
  --note "This proof validates composition/readiness semantics. Dedicated formal, cryptographic, performance, and multi-cloud proofs remain the evidence for their narrower claims."

echo "$out"
exit "$rc"
