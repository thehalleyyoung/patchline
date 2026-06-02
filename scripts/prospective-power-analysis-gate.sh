#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/prospective-power-analysis-gate.json}"; OUT="${2:-results/generated/prospective-power-analysis}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.prospective-power-analysis-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "prospective power analysis" "make prospective-power-analysis-gate"; do grep -F "$phrase" docs/prospective-power-analysis.md README.md > /dev/null; done
bash scripts/prospective-power-analysis.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.prospective-power-analysis/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.prospective-power-analysis-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "prospective-power-analysis gate passed: every item scored with evidence on real self-data, unsupported item rejected"
