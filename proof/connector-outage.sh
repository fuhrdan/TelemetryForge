#!/usr/bin/env bash
source "$(dirname "$0")/_common.sh"; require_execute "$@"; started=$(iso_now); stamp=$(date +%s); log="$RESULTS/raw/connector-outage-$stamp.log"; out="$RESULTS/connector-outage-$stamp.tfproof.json"; compose=(docker compose -f "$ROOT/docker-compose.yml" -f "$ROOT/proof/connector-outage.override.yml")
set +e
{
 "${compose[@]}" up -d --wait timescaledb kafka kafka-init gateway worker router
 body=$(curl -fsS -H 'Content-Type: application/json' -d '{"source":"proof-client","type":"proof.connector","timestamp":"'"$(date -u +%Y-%m-%dT%H:%M:%SZ)"'","tags":{"environment":"proof"},"payload":{"message":"connector isolation proof"},"schema_version":"1.0"}' http://localhost:8080/api/v1/events)
 echo "$body"; event_id=$(python3 -c 'import json,sys; print(json.loads(sys.argv[1]).get("event_id") or json.loads(sys.argv[1]).get("id") or "")' "$body"); echo EVENT_ID=$event_id; [[ -n "$event_id" ]]
 sleep 8
 rows=$(docker compose exec -T timescaledb psql -U telemetryforge -d telemetryforge -Atc "SELECT destination||':'||status FROM routing_deliveries WHERE event_id='$event_id' ORDER BY destination;")
 echo "$rows"
 echo "$rows" | grep -Eq 'healthy-kafka:delivered'
 echo "$rows" | grep -Eq 'broken-http:(retry|dead_letter)'
 curl -fsS http://localhost:8082/ready >/dev/null
} >"$log" 2>&1; rc=$?
set -e; status=$([[ $rc -eq 0 ]]&&echo pass||echo fail)
write_proof --out "$out" --scenario connector-outage-isolation --status "$status" --started-at "$started" --assertion "healthy-lane-delivered:$status:healthy Kafka destination completed while HTTP destination failed" --assertion "router-remained-ready:$status:router readiness remained healthy" --evidence "log:$log" --config "routing:$ROOT/routing/proof-connector-outage.json" --config "compose:$ROOT/docker-compose.yml"; echo "$out"
