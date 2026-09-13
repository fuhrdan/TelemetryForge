#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/_common.sh"
require_execute "$@"
started=$(iso_now)
stamp=$(date +%s)
log="$RESULTS/raw/autonomous-control-$stamp.log"
report="$RESULTS/raw/autonomy-check-$stamp.json"
out="$RESULTS/autonomous-control-$stamp.tfproof.json"

set +e
{
  go test -count=1 ./internal/autonomy ./internal/shaping ./internal/wasmplugin
  go run ./cmd/autonomycheck --output "$report"
  go run ./cmd/plugincheck --manifest plugins/examples/noop-predictor.manifest.json --module plugins/examples/noop-predictor.wasm
} >"$log" 2>&1
rc=$?
set -e
status=$([[ $rc -eq 0 ]] && echo pass || echo fail)

write_proof --out "$out" --scenario autonomous-control --status "$status" --started-at "$started" \
  --assertion "shadow-no-mutation:$status:shadow mode predicts an action while leaving the production multiplier at 1.0" \
  --assertion "bounded-auto-action:$status:auto mode can apply only the bounded temporary non-protected multiplier" \
  --assertion "guarded-rollback:$status:error-rate regression restores the multiplier to 1.0 and records terminal evidence" \
  --assertion "protected-telemetry:$status:shaping tests confirm protected telemetry remains kept under an autonomy multiplier" \
  --assertion "wasm-boundary:$status:example module identity and exported entrypoint pass the capability-free plugin validator" \
  --evidence "log:$log" \
  --evidence "autonomy-report:$report" \
  --config "autonomy-controller:$ROOT/internal/autonomy/controller.go" \
  --config "shaping-engine:$ROOT/internal/shaping/engine.go" \
  --config "wasm-manifest:$ROOT/plugins/examples/noop-predictor.manifest.json" \
  --note "This proof validates bounded controller semantics. It is not evidence of a universal ML model, production savings, or safe execution of arbitrary third-party Wasm engines."

echo "$out"
exit "$rc"
