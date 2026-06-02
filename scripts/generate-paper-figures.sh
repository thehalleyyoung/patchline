#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if [[ $# -lt 2 ]]; then
  echo "usage: scripts/generate-paper-figures.sh analysis-dir[,analysis-dir...] out-dir [--json]" >&2
  exit 2
fi

analyses="$1"
out="$2"
shift 2

go run ./cmd/patchline repo figures \
  --analyses "$analyses" \
  --out "$out" \
  "$@"
