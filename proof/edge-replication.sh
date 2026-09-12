#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/_common.sh"
require_execute "$@"
started=$(iso_now); stamp=$(date +%s); log="$RESULTS/raw/edge-replication-$stamp.log"; out="$RESULTS/edge-replication-$stamp.tfproof.json"
set +e
go test -count=1 -v ./internal/replication >"$log" 2>&1
rc=$?
set -e
status=$([[ $rc -eq 0 ]] && echo pass || echo fail)
write_proof --out "$out" --scenario edge-replicated-durability --status "$status" --started-at "$started" \
  --assertion "quorum-domain-policy:$status:regional/cross-region/cross-cloud policies require the requested failure-domain diversity" \
  --assertion "replica-fsync-idempotency:$status:exact retries are idempotent and conflicting origin sequences fail closed" \
  --assertion "replica-crash-recovery:$status:valid receiver records survive reopen and incomplete tails are truncated" \
  --assertion "bounded-replica-lifecycle:$status:receiver capacity fails closed and released downstream checkpoints can reclaim replica space" \
  --assertion "authenticated-peer-api:$status:replication endpoints enforce the configured bearer-token boundary" \
  --evidence "log:$log" \
  --config "replication-manager:$ROOT/internal/replication/manager.go" \
  --config "replica-store:$ROOT/internal/replication/store.go" \
  --config "replication-policy:$ROOT/internal/replication/policy.go"
echo "$out"; exit "$rc"
