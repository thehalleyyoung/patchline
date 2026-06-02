#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/resource-budgets-gate.json}"
OUT="${2:-results/generated/resource-budgets-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.resource-budgets-gate/v1" and (.claim|length) > 200 and (.budgets|length) >= 1' "$SPEC" > /dev/null

for phrase in "resource budget" "over-budget" "make resource-budgets-gate"; do
  grep -F "$phrase" docs/resource-budgets.md README.md > /dev/null
done

bash scripts/resource-budgets.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in resource-budgets.json resource-budgets.md README.md; do
  test -s "$OUT/$output"
done

# Within-budget run admitted; over-budget run rejected at exactly the analyze stage
# for the memory resource; the rejection names the offending stage and resource.
jq -e '
  .version == "patchline.resource-budgets/v1" and
  .within_budget.admitted == true and
  .over_budget.admitted == false and
  (.over_budget.overruns | length) == 1 and
  (.over_budget.overruns[0].stage == "analyze") and
  (.over_budget.overruns[0].over | index("memory_mb") != null)
' "$OUT/resource-budgets.json" > /dev/null

jq -n --slurpfile r "$OUT/resource-budgets.json" '{
  version: "patchline.resource-budgets-gate-results/v1",
  within_admitted: $r[0].within_budget.admitted,
  over_admitted: $r[0].over_budget.admitted,
  offending_stage: $r[0].over_budget.overruns[0].stage,
  verified: true
}' > "$OUT/gate-summary.json"

echo "resource-budgets gate passed: within-budget admitted, over-budget rejected at analyze/memory_mb"
