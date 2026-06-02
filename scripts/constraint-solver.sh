#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/constraint-solver-gate.json}"
OUT="${2:-results/generated/constraint-solver}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.constraint-solver-gate/v1"' "$SPEC" > /dev/null

jq '
  def check_not_null($col; $rows):
    ([ $rows[] | select(.[$col] == null) ]) as $bad
    | {satisfiable: (($bad | length) == 0), counterexample: ($bad | first)};
  def check_fk($col; $allowed; $rows):
    ([ $rows[] | select([ .[$col] == $allowed[] ] | any | not) ]) as $bad
    | {satisfiable: (($bad | length) == 0), counterexample: ($bad | first)};
  {
    version: "patchline.constraint-solver/v1",
    not_null_good: check_not_null(.not_null_column; .good_rows),
    not_null_bad: check_not_null(.not_null_column; .bad_rows),
    fk_good: check_fk(.fk_column; .fk_allowed; .fk_good_rows),
    fk_bad: check_fk(.fk_column; .fk_allowed; .fk_bad_rows)
  }
' "$SPEC" > "$OUT/solver.json"

{
  echo "# Constraint-solver obligations"
  echo
  echo "NOT NULL good satisfiable: $(jq -r '.not_null_good.satisfiable' "$OUT/solver.json")"
  echo "NOT NULL bad counterexample: $(jq -rc '.not_null_bad.counterexample' "$OUT/solver.json")"
  echo "FK bad counterexample: $(jq -rc '.fk_bad.counterexample' "$OUT/solver.json")"
} > "$OUT/solver.md"
cp "$OUT/solver.md" "$OUT/README.md"

echo "constraint-solver worker: nn_good=$(jq -r '.not_null_good.satisfiable' "$OUT/solver.json") nn_bad_ce=$(jq -rc '.not_null_bad.counterexample.id' "$OUT/solver.json") fk_bad_ce=$(jq -rc '.fk_bad.counterexample.id' "$OUT/solver.json")"
