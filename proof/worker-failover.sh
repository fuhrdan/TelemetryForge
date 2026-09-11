#!/usr/bin/env bash
source "$(dirname "$0")/_common.sh"; require_execute "$@"; started=$(iso_now); stamp=$(date +%s); log="$RESULTS/raw/worker-failover-$stamp.log"; out="$RESULTS/worker-failover-$stamp.tfproof.json"
set +e
{
 docker compose up -d --wait timescaledb kafka kafka-init gateway
 docker compose up -d --scale worker=2 worker; sleep 8
 ids=( $(docker compose ps -q worker) ); echo WORKERS=${#ids[@]}; [[ ${#ids[@]} -ge 2 ]]; docker stop "${ids[0]}"; sleep 3
 running=$(docker compose ps --status running -q worker|wc -l|tr -d ' '); echo RUNNING_AFTER_STOP=$running; [[ $running -ge 1 ]]
 body=$(curl -fsS -H 'Content-Type: application/json' -d '{"source":"proof-worker","type":"audit.worker-failover","timestamp":"'"$(date -u +%Y-%m-%dT%H:%M:%SZ)"'","tags":{"severity":"error"},"schema_version":"1.0"}' http://localhost:8080/api/v1/events); echo "$body"; event_id=$(python3 -c 'import json,sys; d=json.loads(sys.argv[1]); print(d.get("event_id") or d.get("id") or "")' "$body"); [[ -n "$event_id" ]]
 persisted=0; for _ in $(seq 1 60); do persisted=$(docker compose exec -T timescaledb psql -U telemetryforge -d telemetryforge -Atc "SELECT count(*) FROM telemetry_events WHERE event_id='$event_id';" 2>/dev/null || echo 0); [[ "$persisted" == "1" ]] && break; sleep 1; done
 echo PERSISTED=$persisted; [[ "$persisted" == "1" ]]
} >"$log" 2>&1; rc=$?
set -e; status=$([[ $rc -eq 0 ]]&&echo pass||echo fail)
write_proof --out "$out" --scenario worker-failover --status "$status" --started-at "$started" --assertion "worker-remained:$status:at least one worker remained running" --assertion "accepted-event-persisted:$status:remaining consumer processed an accepted protected event" --evidence "log:$log" --config "compose:$ROOT/docker-compose.yml"; echo "$out"
