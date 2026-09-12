#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/_common.sh"
require_execute "$@"
started=$(iso_now); stamp=$(date +%s); log="$RESULTS/raw/wal-crash-recovery-$stamp.log"; out="$RESULTS/wal-crash-recovery-$stamp.tfproof.json"
set +e
go test -count=1 -v ./internal/wal >"$log" 2>&1
rc=$?
set -e
status=$([[ $rc -eq 0 ]] && echo pass || echo fail)
write_proof --out "$out" --scenario wal-crash-recovery --status "$status" --started-at "$started" \
  --assertion "durable-recovery:$status:fsynced records survive reopen and incomplete tails are truncated" \
  --assertion "corruption-fails-closed:$status:completed-frame corruption is rejected" \
  --assertion "pressure-before-drop:$status:capacity rejects new acceptance before overwrite" \
  --assertion "sequence-continuity:$status:edge and source sequences survive restart/compaction" \
  --evidence "log:$log" --config "wal-store:$ROOT/internal/wal/store.go" --config "wal-model:$ROOT/internal/wal/model.go"
echo "$out"; exit "$rc"
