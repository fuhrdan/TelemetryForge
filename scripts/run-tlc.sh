#!/usr/bin/env bash
set -euo pipefail
ROOT=$(cd "$(dirname "$0")/.." && pwd)
JAR=${TLA2TOOLS_JAR:-}
if [[ -z "$JAR" ]]; then
  echo "TLA2TOOLS_JAR must point to a tla2tools.jar" >&2
  exit 2
fi
if [[ ! -f "$JAR" ]]; then
  echo "TLA2TOOLS_JAR does not exist: $JAR" >&2
  exit 2
fi

EXPECTED_SHA256=$(tr -d '[:space:]' < "$ROOT/formal/tla2tools.sha256")
if command -v sha256sum >/dev/null 2>&1; then
  ACTUAL_SHA256=$(sha256sum "$JAR" | awk '{print $1}')
  if [[ "$ACTUAL_SHA256" != "$EXPECTED_SHA256" ]]; then
    echo "tla2tools.jar digest mismatch: expected $EXPECTED_SHA256, got $ACTUAL_SHA256" >&2
    exit 2
  fi
fi

models=(DurableIngest ReplicatedDurability MeshFailover CryptographicLineage)
for model in "${models[@]}"; do
  echo "== SANY: $model =="
  java -cp "$JAR" tla2sany.SANY "$ROOT/formal/$model.tla"
  echo "== TLC: $model =="
  java -XX:+UseParallelGC -cp "$JAR" tlc2.TLC \
    -cleanup \
    -workers auto \
    -config "$ROOT/formal/models/$model.cfg" \
    "$ROOT/formal/$model.tla"
done
