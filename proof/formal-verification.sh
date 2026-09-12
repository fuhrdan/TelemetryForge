#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/_common.sh"
require_execute "$@"
started=$(iso_now)
stamp=$(date +%s)
log="$RESULTS/raw/formal-verification-$stamp.log"
report="$RESULTS/raw/formal-model-check-$stamp.json"
out="$RESULTS/formal-verification-$stamp.tfproof.json"

set +e
{
  echo "== Go bounded state-space checker =="
  go test -count=1 ./internal/formal
  go run ./cmd/formalcheck --output "$report"
  if [[ -n "${TLA2TOOLS_JAR:-}" ]]; then
    echo "== TLA+ TLC =="
    scripts/run-tlc.sh
  else
    echo "TLA2TOOLS_JAR is not set; TLC phase not executed by this proof invocation." >&2
    exit 3
  fi
} >"$log" 2>&1
rc=$?
set -e
status=$([[ $rc -eq 0 ]] && echo pass || echo fail)

write_proof --out "$out" --scenario formal-verification --status "$status" --started-at "$started" \
  --assertion "durable-ingest:$status:client success never precedes durable evidence and undelivered acknowledged data remains durable" \
  --assertion "replicated-durability:$status:replicated success requires quorum and release follows downstream delivery" \
  --assertion "mesh-failover:$status:bounded owner attempts prevent forwarding loops and terminal failure means no eligible route" \
  --assertion "cryptographic-lineage:$status:tampered sealed history cannot verify in the bounded lineage model" \
  --assertion "dual-checkers:$status:dependency-free Go state exploration and pinned TLC model checking both completed" \
  --evidence "log:$log" \
  --evidence "model-report:$report" \
  --config "formal-readme:$ROOT/formal/README.md" \
  --config "durable-tla:$ROOT/formal/DurableIngest.tla" \
  --config "replication-tla:$ROOT/formal/ReplicatedDurability.tla" \
  --config "mesh-tla:$ROOT/formal/MeshFailover.tla" \
  --config "lineage-tla:$ROOT/formal/CryptographicLineage.tla"

echo "$out"
exit "$rc"
