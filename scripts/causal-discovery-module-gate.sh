#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/causal-discovery-module-gate.json}"; OUT="${2:-results/generated/causal-discovery-module}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.causal-discovery-module-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "causal" "make causal-discovery-module-gate"; do grep -F "$phrase" docs/causal-discovery-module.md README.md > /dev/null; done
bash scripts/causal-discovery-module.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.causal-discovery-module/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.causal-discovery-module-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "causal-discovery-module gate passed: every causal edge evidence-backed, spurious correlation rejected"
