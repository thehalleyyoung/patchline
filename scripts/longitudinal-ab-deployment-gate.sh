#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/longitudinal-ab-deployment-gate.json}"; OUT="${2:-results/generated/longitudinal-ab-deployment}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.longitudinal-ab-deployment-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "incident-rate" "make longitudinal-ab-deployment-gate"; do grep -F "$phrase" docs/longitudinal-ab-deployment.md README.md > /dev/null; done
bash scripts/longitudinal-ab-deployment.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.longitudinal-ab-deployment/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.longitudinal-ab-deployment-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "longitudinal-ab-deployment gate passed: treated arm beats control every period, underperforming period rejected"
