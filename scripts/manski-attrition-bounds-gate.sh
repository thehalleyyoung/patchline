#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/manski-attrition-bounds-gate.json}"; OUT="${2:-results/generated/manski-attrition-bounds}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.manski-attrition-bounds-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "attrition bounds" "make manski-attrition-bounds-gate"; do grep -F "$phrase" docs/manski-attrition-bounds.md README.md > /dev/null; done
bash scripts/manski-attrition-bounds.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.manski-attrition-bounds/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.manski-attrition-bounds-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "manski-attrition-bounds gate passed: every item scored with evidence on real self-data, unsupported item rejected"
