#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/causal-effect-estimate-gate.json}"; OUT="${2:-results/generated/causal-effect-estimate}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.causal-effect-estimate-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "confounder" "make causal-effect-estimate-gate"; do grep -F "$phrase" docs/causal-effect-estimate.md README.md > /dev/null; done
bash scripts/causal-effect-estimate.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.causal-effect-estimate/v1" and .is_reduction==true and .adjusted_lt_naive_magnitude==true and .naive_biased==true' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.causal-effect-estimate-gate-results/v1",adjusted_effect:$r[0].adjusted_effect,is_reduction:$r[0].is_reduction,naive_biased:$r[0].naive_biased,verified:true}' > "$OUT/gate-summary.json"
echo "causal-effect-estimate gate passed: confounder-adjusted effect is a reduction, naive estimate flagged biased"
