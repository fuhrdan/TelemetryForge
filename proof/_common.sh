#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RESULTS="${ROOT}/proof/results"
mkdir -p "${RESULTS}/raw"
require_execute(){ [[ " ${*} " == *" --execute "* ]] || { echo "Refusing disruptive proof run without --execute" >&2; exit 2; }; }
iso_now(){ date -u +%Y-%m-%dT%H:%M:%SZ; }
write_proof(){ python3 "${ROOT}/scripts/proof_common.py" "$@"; }
