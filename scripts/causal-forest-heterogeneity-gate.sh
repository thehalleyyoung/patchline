#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/causal-forest-heterogeneity-gate.json}"; OUT="${2:-results/generated/causal-forest-heterogeneity}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.causal-forest-heterogeneity-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "causal forest" "make causal-forest-heterogeneity-gate"; do grep -F "$phrase" docs/causal-forest-heterogeneity.md README.md > /dev/null; done
bash scripts/causal-forest-heterogeneity.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.causal-forest-heterogeneity/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.causal-forest-heterogeneity-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "causal-forest-heterogeneity gate passed: every item scored with evidence on real self-data, unsupported item rejected"
