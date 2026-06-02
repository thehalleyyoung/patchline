#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/saas-reference-deployment-gate.json}"; OUT="${2:-results/generated/saas-reference-deployment}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.saas-reference-deployment-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "SLO" "make saas-reference-deployment-gate"; do grep -F "$phrase" docs/saas-reference-deployment.md README.md > /dev/null; done
bash scripts/saas-reference-deployment.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.saas-reference-deployment/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.saas-reference-deployment-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "saas-reference-deployment gate passed: every SLO met, breached SLO rejected"
