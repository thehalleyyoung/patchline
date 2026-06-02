#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/audited-incident-dashboard-gate.json}"; OUT="${2:-results/generated/audited-incident-dashboard}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.audited-incident-dashboard-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "independently audited" "make audited-incident-dashboard-gate"; do grep -F "$phrase" docs/audited-incident-dashboard.md README.md > /dev/null; done
bash scripts/audited-incident-dashboard.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.audited-incident-dashboard/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.audited-incident-dashboard-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "audited-incident-dashboard gate passed: every item scored with evidence on real self-data, unsupported item rejected"
