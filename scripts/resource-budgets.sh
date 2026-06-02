#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/resource-budgets-gate.json}"
OUT="${2:-results/generated/resource-budgets}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.resource-budgets-gate/v1" and (.budgets|length) >= 1' "$SPEC" > /dev/null

# Evaluate a run against the per-stage budgets, reporting the first overrun (if any).
jq '
  .budgets as $b
  | def eval($run):
      [ $run[] as $s
        | ($b[] | select(.stage == $s.stage)) as $lim
        | { stage: $s.stage,
            over: ([ if $s.seconds > $lim.seconds then "seconds" else empty end,
                     if $s.memory_mb > $lim.memory_mb then "memory_mb" else empty end,
                     if $s.files > $lim.files then "files" else empty end ]) } ]
      | { overruns: [ .[] | select(.over | length > 0) ] }
      | . + { admitted: ((.overruns | length) == 0) };
  {
    version: "patchline.resource-budgets/v1",
    within_budget: eval(.within_budget_run),
    over_budget: eval(.over_budget_run)
  }
' "$SPEC" > "$OUT/resource-budgets.json"

{
  echo "# Resource budget evaluation"
  echo
  echo "Within-budget run admitted: $(jq -r '.within_budget.admitted' "$OUT/resource-budgets.json")"
  echo
  echo "Over-budget run admitted: $(jq -r '.over_budget.admitted' "$OUT/resource-budgets.json")"
  echo
  echo "Over-budget overruns:"
  jq -r '.over_budget.overruns[] | "- stage `\(.stage)` overran: \(.over | join(", "))"' "$OUT/resource-budgets.json"
} > "$OUT/resource-budgets.md"
cp "$OUT/resource-budgets.md" "$OUT/README.md"

echo "resource-budgets worker: within admitted=$(jq -r '.within_budget.admitted' "$OUT/resource-budgets.json"), over admitted=$(jq -r '.over_budget.admitted' "$OUT/resource-budgets.json")"
