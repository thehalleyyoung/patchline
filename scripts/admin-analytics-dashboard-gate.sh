#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/admin-analytics-dashboard-gate.json}"; OUT="${2:-results/generated/admin-analytics-dashboard}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.admin-analytics-dashboard-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "prevented-incident" "make admin-analytics-dashboard-gate"; do grep -F "$phrase" docs/admin-analytics-dashboard.md README.md > /dev/null; done
bash scripts/admin-analytics-dashboard.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.admin-analytics-dashboard/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.admin-analytics-dashboard-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "admin-analytics-dashboard gate passed: every dashboard metric estimate-backed, unbacked metric rejected"
