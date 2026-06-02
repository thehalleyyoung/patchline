#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/bayesian-hierarchical-model-gate.json}"; OUT="${2:-results/generated/bayesian-hierarchical-model}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.bayesian-hierarchical-model-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "posterior predictive" "make bayesian-hierarchical-model-gate"; do grep -F "$phrase" docs/bayesian-hierarchical-model.md README.md > /dev/null; done
bash scripts/bayesian-hierarchical-model.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.bayesian-hierarchical-model/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.bayesian-hierarchical-model-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "bayesian-hierarchical-model gate passed: every item scored with evidence on real self-data, unsupported item rejected"
