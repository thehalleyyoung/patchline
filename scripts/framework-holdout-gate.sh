#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/framework-holdout-gate.json}"; OUT="${2:-results/generated/framework-holdout}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.framework-holdout-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "generalization" "make framework-holdout-gate"; do grep -F "$phrase" docs/framework-holdout.md README.md > /dev/null; done
bash scripts/framework-holdout.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '
  .version == "patchline.framework-holdout/v1" and
  .no_leakage == true and .evaluated_on == "prisma" and
  (.threshold_selected_on | index("prisma") | not) and
  .leaked_no_leakage == false
' "$OUT/holdout.json" > /dev/null
jq -n --slurpfile r "$OUT/holdout.json" '{version:"patchline.framework-holdout-gate-results/v1", evaluated_on:$r[0].evaluated_on, no_leakage:$r[0].no_leakage, leak_detected:($r[0].leaked_no_leakage|not), verified:true}' > "$OUT/gate-summary.json"
echo "framework-holdout gate passed: threshold selected without holdout, evaluated on holdout, leak detected"
