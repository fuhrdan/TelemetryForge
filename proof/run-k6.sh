#!/usr/bin/env bash
source "$(dirname "$0")/_common.sh"; scenario=${1:-smoke}; require_execute "$@"; case "$scenario" in smoke) js=ingest-smoke.js;; sustained) js=ingest-sustained.js;; backpressure) js=backpressure.js;; *) echo bad scenario >&2; exit 2;; esac; started=$(iso_now); stamp=$(date +%s); log="$RESULTS/raw/k6-$scenario-$stamp.log"; summary="$RESULTS/raw/k6-$scenario-$stamp-summary.json"; out="$RESULTS/k6-$scenario-$stamp.tfproof.json"; t0=$(date +%s)
set +e
docker run --rm --add-host host.docker.internal:host-gateway -v "$ROOT/load/k6:/scripts:ro" -v "$RESULTS/raw:/results" grafana/k6:2.2.0 run --summary-export="/results/$(basename "$summary")" "/scripts/$js" >"$log" 2>&1
rc=$?; set -e; dur=$(($(date +%s)-t0)); status=$([[ $rc -eq 0 ]]&&echo pass||echo fail)
measure_args=(--measurement "wall_duration_seconds:$dur:s")
evidence_args=(--evidence "k6-log:$log")
if [[ -f "$summary" ]]; then
  while IFS= read -r m; do [[ -n "$m" ]] && measure_args+=(--measurement "$m"); done < <(python3 - "$summary" <<'PY'
import json, math, sys
m=json.load(open(sys.argv[1])).get('metrics',{})
def emit(name,value,unit):
    if isinstance(value,(int,float)) and math.isfinite(value): print(f'{name}:{value}:{unit}')
http=m.get('http_reqs',{}); emit('http_requests',http.get('count'),'count'); emit('http_requests_per_second',http.get('rate'),'req/s')
dur=m.get('http_req_duration',{}); emit('http_request_p95_ms',dur.get('p(95)'),'ms')
fail=m.get('http_req_failed',{}); emit('http_request_failed_ratio',fail.get('value'),'ratio')
PY
)
fi
[[ -f "$summary" ]] && evidence_args+=(--evidence "k6-summary:$summary")
write_proof --out "$out" --scenario "k6-$scenario" --status "$status" --started-at "$started" --assertion "k6-thresholds:$status:k6 process and configured thresholds completed" "${measure_args[@]}" "${evidence_args[@]}" --config "k6-script:$ROOT/load/k6/$js" --config "compose:$ROOT/docker-compose.yml" --note 'Measurements are emitted only when present in the executed k6 summary; absent metrics are not synthesized.'
echo "$out"
