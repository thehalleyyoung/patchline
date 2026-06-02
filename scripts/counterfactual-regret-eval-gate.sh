#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/counterfactual-regret-eval-gate.json}"; OUT="${2:-results/generated/counterfactual-regret-eval}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.counterfactual-regret-eval-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "counterfactual regret" "make counterfactual-regret-eval-gate"; do grep -F "$phrase" docs/counterfactual-regret-eval.md README.md > /dev/null; done
bash scripts/counterfactual-regret-eval.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.counterfactual-regret-eval/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.counterfactual-regret-eval-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "counterfactual-regret-eval gate passed: every item scored with evidence on real self-data, unsupported item rejected"
