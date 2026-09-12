#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/_common.sh"
require_execute "$@"
started=$(iso_now); stamp=$(date +%s); log="$RESULTS/raw/mesh-failover-$stamp.log"; out="$RESULTS/mesh-failover-$stamp.tfproof.json"
set +e
go test -count=1 -v ./internal/mesh >"$log" 2>&1
rc=$?
set -e
status=$([[ $rc -eq 0 ]] && echo pass || echo fail)
write_proof --out "$out" --scenario mesh-failover --status "$status" --started-at "$started" \
  --assertion "deterministic-ownership:$status:rendezvous hashing keeps a key on a stable eligible owner" \
  --assertion "health-failover:$status:draining stale pressured or unhealthy nodes are excluded from new routes" \
  --assertion "identity-guard:$status:peer advertisements and forwarding targets must match configured node identity" \
  --assertion "loop-prevention:$status:remote forwarding terminates at the receiving local downstream publisher" \
  --evidence "log:$log" --config "mesh-manager:$ROOT/internal/mesh/manager.go" --config "mesh-http:$ROOT/internal/mesh/http.go"
echo "$out"; exit "$rc"
