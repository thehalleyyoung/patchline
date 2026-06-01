#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

commands=(
  "semantics-audit"
  "trace-reconstruct"
  "analyze-migration"
  "solver-obligations"
  "repair-semantics"
  "archive-query"
  "semantic-regressions"
  "benchmark-suite"
  "artifact-ground-truth"
  "phase-check"
  "artifact-baselines"
  "artifact-ablations"
  "artifact-scale"
  "artifact-tables"
  "artifact-numbers"
  "artifact-subtasks"
  "artifact-corpus-audit"
  "artifact-provenance"
  "artifact-benchmark"
)

for command in "${commands[@]}"; do
  if ! grep -Eq "case \"([^\"]*, )?${command}(\"|\",)" cmd/patchline/main.go; then
    echo "artifact command is not implemented in cmd/patchline/main.go: $command" >&2
    exit 1
  fi
done

echo "artifact commands present: ${commands[*]}"
