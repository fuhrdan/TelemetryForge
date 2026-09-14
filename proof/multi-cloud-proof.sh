#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/_common.sh"
require_execute "$@"
started=$(iso_now)
stamp=$(date +%s)
log="$RESULTS/raw/multi-cloud-proof-$stamp.log"
report="$RESULTS/raw/multi-cloud-check-$stamp.json"
out="$RESULTS/multi-cloud-proof-$stamp.tfproof.json"
events="${TELEMETRYFORGE_MULTICLOUD_PROOF_EVENTS:-10000}"

set +e
{
  go test -race -count=1 ./internal/multicloudproof
  go run ./cmd/multicloudcheck --events "$events" --output "$report"
  python3 - "$report" <<'PY'
import json, sys
r=json.load(open(sys.argv[1], encoding='utf-8'))
assert r['passed'] is True
assert r['total_lost'] == 0
assert r['total_corrupted'] == 0
assert r['total_unaccounted'] == 0
assert len(r['scenarios']) == 6
assert any(s['name']=='disk-pressure-backpressure' and s['rejected_under_load'] > 0 for s in r['scenarios'])
assert any(s['name']=='packet-loss-latency' and s['after_recovery']['duplicate_attempts'] > 0 for s in r['scenarios'])
print('validated multi-cloud accounting report')
PY
} >"$log" 2>&1
rc=$?
set -e
status=$([[ $rc -eq 0 ]] && echo pass || echo fail)

read -r offered accepted rejected duplicates lost corrupted unaccounted < <(python3 - "$report" <<'PY'
import json, sys
try:
    r=json.load(open(sys.argv[1], encoding='utf-8'))
    print(r['total_offered'],r['total_accepted'],r['total_rejected'],r['total_duplicate_attempts'],r['total_lost'],r['total_corrupted'],r['total_unaccounted'])
except Exception:
    print(0,0,0,0,0,0,0)
PY
)

write_proof --out "$out" --scenario multi-cloud-proof --status "$status" --started-at "$started" \
  --assertion "accepted-accounting:$status:every accepted event remains uniquely delivered retained or explicitly lost; unaccounted must remain zero" \
  --assertion "zero-model-loss:$status:named fault schedules report zero lost and zero corrupted accepted events" \
  --assertion "backpressure-before-accept:$status:capacity collapse below durability quorum rejects new events before acknowledgement" \
  --assertion "ambiguous-delivery-accounting:$status:packet-loss/latency ambiguity may create duplicate attempts but not duplicate logical delivery" \
  --assertion "partition-retention:$status:when every delivery path is partitioned accepted events remain durably retained rather than unaccounted" \
  --assertion "recovery-drain:$status:after healing the modeled fabric all accepted events become uniquely delivered" \
  --measurement "offered-events:$offered:events" \
  --measurement "accepted-events:$accepted:events" \
  --measurement "rejected-before-acceptance:$rejected:events" \
  --measurement "duplicate-attempts:$duplicates:attempts" \
  --measurement "lost-events:$lost:events" \
  --measurement "corrupted-events:$corrupted:events" \
  --measurement "unaccounted-events:$unaccounted:events" \
  --evidence "log:$log" \
  --evidence "multi-cloud-report:$report" \
  --config "fault-model:$ROOT/internal/multicloudproof/model.go" \
  --config "multi-cloud-topology:$ROOT/deployments/multicloud/docker-compose.override.yml" \
  --note "This proof is a deterministic failure-domain/accounting model. It does not claim that CI caused a real AWS, GCP, or Azure outage. Real-provider destructive tests require an isolated user-owned environment and provider-specific fault injection."

echo "$out"
exit "$rc"
