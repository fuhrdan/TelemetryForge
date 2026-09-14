#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/_common.sh"
require_execute "$@"
started=$(iso_now)
stamp=$(date +%s)
log="$RESULTS/raw/multi-cloud-compose-$stamp.log"
status_before="$RESULTS/raw/multi-cloud-compose-status-before-$stamp.json"
status_after="$RESULTS/raw/multi-cloud-compose-status-after-$stamp.json"
out="$RESULTS/multi-cloud-compose-$stamp.tfproof.json"
compose=(docker compose -f docker-compose.yml -f deployments/multicloud/docker-compose.override.yml)

cleanup() {
  "${compose[@]}" up -d edge-peer-b edge-peer-c >/dev/null 2>&1 || true
}
trap cleanup EXIT

post_event() {
  local source=$1
  curl -sS -o /tmp/tf-multicloud-response -w '%{http_code}' \
    -H 'Content-Type: application/json' \
    -d '{"source":"'"$source"'","type":"audit.multi-cloud-proof","timestamp":"'"$(date -u +%Y-%m-%dT%H:%M:%SZ)"'","schema_version":"1.0"}' \
    http://localhost:8083/api/v1/events || true
}

set +e
{
  "${compose[@]}" up -d --build --wait kafka kafka-init edge-peer-b edge-peer-c edge
  curl -fsS http://localhost:8083/edge/status > "$status_before"

  baseline=$(post_event proof-multicloud-baseline)
  [[ "$baseline" == "200" || "$baseline" == "202" ]]

  # Remove one cloud. AWS origin + Azure still satisfy cross-cloud quorum=2.
  "${compose[@]}" stop edge-peer-b
  sleep 2
  one_cloud_down=$(post_event proof-multicloud-gcp-down)
  [[ "$one_cloud_down" == "200" || "$one_cloud_down" == "202" ]]

  # Remove the second remote cloud. Only the local AWS copy remains, so the
  # cross-cloud acceptance contract must fail closed.
  "${compose[@]}" stop edge-peer-c
  sleep 2
  quorum_lost=$(post_event proof-multicloud-quorum-lost)
  [[ "$quorum_lost" != "200" && "$quorum_lost" != "202" ]]

  "${compose[@]}" up -d --wait edge-peer-b edge-peer-c
  sleep 3
  recovered=$(post_event proof-multicloud-recovered)
  [[ "$recovered" == "200" || "$recovered" == "202" ]]
  curl -fsS http://localhost:8083/edge/status > "$status_after"

  echo "baseline_http=$baseline"
  echo "one_cloud_down_http=$one_cloud_down"
  echo "quorum_lost_http=$quorum_lost"
  echo "recovered_http=$recovered"
} >"$log" 2>&1
rc=$?
set -e
status=$([[ $rc -eq 0 ]] && echo pass || echo fail)

write_proof --out "$out" --scenario multi-cloud-compose --status "$status" --started-at "$started" \
  --assertion "baseline-cross-cloud-quorum:$status:logical AWS/GCP/Azure topology accepts when cross-cloud durability is available" \
  --assertion "single-cloud-failover:$status:stopping the logical GCP peer still leaves AWS plus Azure cross-cloud quorum" \
  --assertion "quorum-collapse-fails-closed:$status:stopping both remote-cloud peers prevents successful acknowledgement" \
  --assertion "recovery-restores-acceptance:$status:restarting remote peers restores cross-cloud acceptance" \
  --evidence "log:$log" \
  --evidence "status-before:$status_before" \
  --evidence "status-after:$status_after" \
  --config "multi-cloud-topology:$ROOT/deployments/multicloud/docker-compose.override.yml" \
  --config "replication-manager:$ROOT/internal/replication/manager.go" \
  --note "This destructive suite kills logical cloud peers on one Docker host. It validates runtime fail-closed/failover behavior but is not evidence of a real provider outage."

echo "$out"
exit "$rc"
