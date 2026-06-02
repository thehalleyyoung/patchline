#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/evidence-trace-view-gate.json}"; OUT="${2:-results/generated/evidence-trace-view}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.evidence-trace-view-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "supporting evidence" "make evidence-trace-view-gate"; do grep -F "$phrase" docs/evidence-trace-view.md README.md > /dev/null; done
bash scripts/evidence-trace-view.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.evidence-trace-view/v1" and .resolved==true and .all_grounded==true and .dangling_resolved==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.evidence-trace-view-gate-results/v1",resolved:$r[0].resolved,all_grounded:$r[0].all_grounded,dangling_rejected:($r[0].dangling_resolved|not),verified:true}' > "$OUT/gate-summary.json"
echo "evidence-trace-view gate passed: trace complete and grounded, dangling evidence node rejected"
