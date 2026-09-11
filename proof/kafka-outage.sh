#!/usr/bin/env bash
source "$(dirname "$0")/_common.sh"; require_execute "$@"; started=$(iso_now); stamp=$(date +%s); log="$RESULTS/raw/kafka-outage-$stamp.log"; out="$RESULTS/kafka-outage-$stamp.tfproof.json"
set +e
{
 docker compose up -d --wait timescaledb kafka kafka-init gateway worker
 curl -fsS http://localhost:8080/ready
 docker compose stop kafka; sleep 3
 outage_code=$(curl -sS -o /tmp/tf-kafka-proof-response -w '%{http_code}' -H 'Content-Type: application/json' -d '{"source":"proof-kafka","type":"audit.kafka-outage","timestamp":"'"$(date -u +%Y-%m-%dT%H:%M:%SZ)"'","schema_version":"1.0"}' http://localhost:8080/api/v1/events || true)
 echo OUTAGE_POST_HTTP=$outage_code
 [[ "$outage_code" != "202" ]]
 t0=$(date +%s); docker compose start kafka
 until curl -fsS http://localhost:8080/ready >/dev/null 2>&1; do sleep 1; [[ $(($(date +%s)-t0)) -lt 120 ]] || break; done
 recovery=$(($(date +%s)-t0)); echo RECOVERY_SECONDS=$recovery
 curl -fsS http://localhost:8080/ready
} >"$log" 2>&1; rc=$?
set -e; status=$([[ $rc -eq 0 ]]&&echo pass||echo fail); rec=$(grep 'RECOVERY_SECONDS=' "$log"|tail -1|cut -d= -f2); rec=${rec:-0}; code=$(grep 'OUTAGE_POST_HTTP=' "$log"|tail -1|cut -d= -f2); code=${code:-0}
write_proof --out "$out" --scenario kafka-outage-recovery --status "$status" --started-at "$started" --assertion "no-false-accept:$status:gateway did not return 202 while Kafka was unavailable (HTTP $code)" --assertion "gateway-recovered:$status:gateway readiness returned after Kafka restart" --measurement "recovery_seconds:$rec:s" --evidence "log:$log" --config "compose:$ROOT/docker-compose.yml"; echo "$out"
