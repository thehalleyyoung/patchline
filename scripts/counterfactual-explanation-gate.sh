#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/counterfactual-explanation-gate.json}"; OUT="${2:-results/generated/counterfactual-explanation}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.counterfactual-explanation-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "counterfactual" "make counterfactual-explanation-gate"; do grep -F "$phrase" docs/counterfactual-explanation.md README.md > /dev/null; done
bash scripts/counterfactual-explanation.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.counterfactual-explanation/v1" and .flips==true and .minimal==true and .nonflip_flips==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.counterfactual-explanation-gate-results/v1",flips:$r[0].flips,minimal:$r[0].minimal,nonflip_rejected:($r[0].nonflip_flips|not),verified:true}' > "$OUT/gate-summary.json"
echo "counterfactual-explanation gate passed: counterfactual is sufficient and minimal, non-flipping rejected"
