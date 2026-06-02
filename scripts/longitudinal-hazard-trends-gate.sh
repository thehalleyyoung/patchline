#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/longitudinal-hazard-trends-gate.json}"; OUT="${2:-results/generated/longitudinal-hazard-trends}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.longitudinal-hazard-trends-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "longitudinal hazard trends" "make longitudinal-hazard-trends-gate"; do grep -F "$phrase" docs/longitudinal-hazard-trends.md README.md > /dev/null; done
bash scripts/longitudinal-hazard-trends.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.longitudinal-hazard-trends/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.longitudinal-hazard-trends-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "longitudinal-hazard-trends gate passed: every item scored with evidence on real self-data, unsupported item rejected"
