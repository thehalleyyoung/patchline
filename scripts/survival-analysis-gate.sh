#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/survival-analysis-gate.json}"; OUT="${2:-results/generated/survival-analysis}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.survival-analysis-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "survival" "make survival-analysis-gate"; do grep -F "$phrase" docs/survival-analysis.md README.md > /dev/null; done
bash scripts/survival-analysis.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.survival-analysis/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.survival-analysis-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "survival-analysis gate passed: gating lengthens time-to-incident every cohort, shortening cohort rejected"
