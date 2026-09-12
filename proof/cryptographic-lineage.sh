#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/_common.sh"
require_execute "$@"
started=$(iso_now); stamp=$(date +%s); log="$RESULTS/raw/cryptographic-lineage-$stamp.log"; out="$RESULTS/cryptographic-lineage-$stamp.tfproof.json"
set +e
go test -count=1 -v ./internal/lineage ./internal/wal >"$log" 2>&1
rc=$?
set -e
status=$([[ $rc -eq 0 ]] && echo pass || echo fail)
write_proof --out "$out" --scenario cryptographic-lineage --status "$status" --started-at "$started" \
  --assertion "record-chain:$status:v2 WAL records bind payload digest sequencing metadata and the prior record hash" \
  --assertion "merkle-seal:$status:closed WAL segments produce deterministic Merkle roots and signed seals" \
  --assertion "signature-tamper:$status:modified signed seal metadata fails Ed25519 verification" \
  --assertion "trusted-identity:$status:a supplied public key authenticates the expected edge signer" \
  --assertion "legacy-upgrade:$status:v1 WAL remains readable and can anchor the v2 lineage boundary" \
  --evidence "log:$log" \
  --config "lineage:$ROOT/internal/lineage/lineage.go" \
  --config "wal-lineage:$ROOT/internal/wal/model.go" \
  --config "wal-audit:$ROOT/internal/wal/audit.go"
echo "$out"; exit "$rc"
