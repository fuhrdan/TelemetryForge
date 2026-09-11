#!/usr/bin/env bash
source "$(dirname "$0")/_common.sh"; require_execute "$@"; started=$(iso_now); stamp=$(date +%s); log="$RESULTS/raw/postgres-outage-$stamp.log"; out="$RESULTS/postgres-outage-$stamp.tfproof.json"
set +e
{
 docker compose up -d --wait timescaledb kafka kafka-init gateway worker
 docker compose stop timescaledb; sleep 3
 body=$(curl -sS -w '\nHTTP=%{http_code}' -H 'Content-Type: application/json' -d '{"source":"proof-db","type":"audit.database-outage","timestamp":"'"$(date -u +%Y-%m-%dT%H:%M:%SZ)"'","tags":{"severity":"error"},"schema_version":"1.0"}' http://localhost:8080/api/v1/events)
 echo "$body"; code=$(echo "$body"|sed -n 's/^HTTP=//p'); payload=$(echo "$body"|sed '/^HTTP=/d'); event_id=$(python3 -c 'import json,sys; d=json.loads(sys.stdin.read()); print(d.get("event_id") or d.get("id") or "")' <<<"$payload"); echo EVENT_ID=$event_id; [[ "$code" == "202" && -n "$event_id" ]]
 t0=$(date +%s); docker compose start timescaledb
 until docker compose exec -T timescaledb pg_isready -U telemetryforge -d telemetryforge >/dev/null 2>&1; do sleep 1; [[ $(($(date +%s)-t0)) -lt 120 ]] || break; done
 persisted=0; for _ in $(seq 1 60); do persisted=$(docker compose exec -T timescaledb psql -U telemetryforge -d telemetryforge -Atc "SELECT count(*) FROM telemetry_events WHERE event_id='$event_id';" 2>/dev/null || echo 0); [[ "$persisted" == "1" ]] && break; sleep 1; done
 recovery=$(($(date +%s)-t0)); echo RECOVERY_SECONDS=$recovery PERSISTED=$persisted; [[ "$persisted" == "1" ]]
} >"$log" 2>&1; rc=$?
set -e; status=$([[ $rc -eq 0 ]]&&echo pass||echo fail); rec=$(grep -o 'RECOVERY_SECONDS=[0-9]*' "$log"|tail -1|cut -d= -f2); rec=${rec:-0}
write_proof --out "$out" --scenario postgres-outage-recovery --status "$status" --started-at "$started" --assertion "kafka-accepted:$status:gateway durably accepted the event while PostgreSQL was unavailable" --assertion "event-persisted-after-recovery:$status:accepted event reached TimescaleDB after PostgreSQL resumed" --measurement "recovery_to_persistence_seconds:$rec:s" --evidence "log:$log" --config "compose:$ROOT/docker-compose.yml"; echo "$out"
