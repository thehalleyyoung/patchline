#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/artifact-badge-audit-gate.json}"; OUT="${2:-results/generated/artifact-badge-audit}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.artifact-badge-audit-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "badge" "make artifact-badge-audit-gate"; do grep -F "$phrase" docs/artifact-badge-audit.md README.md > /dev/null; done
bash scripts/artifact-badge-audit.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.artifact-badge-audit/v1" and .all_earned==true and .earn_rate==1 and .unearned_met==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.artifact-badge-audit-gate-results/v1",earn_rate:$r[0].earn_rate,all_earned:$r[0].all_earned,unearned_rejected:($r[0].unearned_met|not),verified:true}' > "$OUT/gate-summary.json"
echo "artifact-badge-audit gate passed: every badge criterion satisfied by evidence, unearned badge rejected"
