#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/global-hazard-prevalence-map-gate.json}"; OUT="${2:-results/generated/global-hazard-prevalence-map}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.global-hazard-prevalence-map-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "prevalence map" "make global-hazard-prevalence-map-gate"; do grep -F "$phrase" docs/global-hazard-prevalence-map.md README.md > /dev/null; done
bash scripts/global-hazard-prevalence-map.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.global-hazard-prevalence-map/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.global-hazard-prevalence-map-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "global-hazard-prevalence-map gate passed: every item scored with evidence on real self-data, unsupported item rejected"
