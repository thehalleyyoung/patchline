#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/quickstart-sixty-seconds-gate.json}"; OUT="${2:-results/generated/quickstart-sixty-seconds}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.quickstart-sixty-seconds-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "sixty seconds" "make quickstart-sixty-seconds-gate"; do grep -F "$phrase" docs/quickstart-sixty-seconds.md README.md > /dev/null; done
bash scripts/quickstart-sixty-seconds.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.quickstart-sixty-seconds/v1" and .within_budget==true and .total_seconds<=60 and .slow_within_budget==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.quickstart-sixty-seconds-gate-results/v1",total_seconds:$r[0].total_seconds,within_budget:$r[0].within_budget,over_budget_flagged:($r[0].slow_within_budget|not),verified:true}' > "$OUT/gate-summary.json"
echo "quickstart-sixty-seconds gate passed: end-to-end under sixty seconds, over-budget run flagged"
