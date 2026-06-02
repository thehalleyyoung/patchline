#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/constraint-solver-gate.json}"
OUT="${2:-results/generated/constraint-solver}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.constraint-solver-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null

for phrase in "counterexample" "make constraint-solver-gate"; do
  grep -F "$phrase" docs/constraint-solver.md README.md > /dev/null
done

bash scripts/constraint-solver.sh "$SPEC" "$OUT" > "$OUT.run.log"

# Satisfiable obligations discharge; unsatisfiable ones return the exact offending row.
jq -e '
  .version == "patchline.constraint-solver/v1" and
  .not_null_good.satisfiable == true and
  .not_null_bad.satisfiable == false and
  (.not_null_bad.counterexample.email == null) and
  (.not_null_bad.counterexample.id == 4) and
  .fk_good.satisfiable == true and
  .fk_bad.satisfiable == false and
  (.fk_bad.counterexample.role == "ghost")
' "$OUT/solver.json" > /dev/null

jq -n --slurpfile r "$OUT/solver.json" '{
  version: "patchline.constraint-solver-gate-results/v1",
  not_null_counterexample: $r[0].not_null_bad.counterexample,
  fk_counterexample: $r[0].fk_bad.counterexample,
  verified: true
}' > "$OUT/gate-summary.json"

echo "constraint-solver gate passed: satisfiable obligations discharged, violations return exact counterexamples"
