#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/community-impact-report-gate.json}"; OUT="${2:-results/generated/community-impact-report}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.community-impact-report-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "gate-backed evidence" "make community-impact-report-gate"; do grep -F "$phrase" docs/community-impact-report.md README.md > /dev/null; done
bash scripts/community-impact-report.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.community-impact-report/v1" and .all_backed==true and .unbacked_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.community-impact-report-gate-results/v1",backed:$r[0].backed,all_backed:$r[0].all_backed,unbacked_rejected:($r[0].unbacked_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "community-impact-report gate passed: every impact metric gate-backed, unbacked metric rejected"
