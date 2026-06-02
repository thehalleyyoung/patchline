#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/config-profiles-gate.json}"; OUT="${2:-results/generated/config-profiles}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.config-profiles-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "strict/balanced/lenient" "make config-profiles-gate"; do grep -F "$phrase" docs/config-profiles.md README.md > /dev/null; done
bash scripts/config-profiles.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.config-profiles/v1" and .recall_ordered==true and .precision_ordered==true and .all_documented==true and .broken_ordered==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.config-profiles-gate-results/v1",recall_ordered:$r[0].recall_ordered,precision_ordered:$r[0].precision_ordered,broken_rejected:($r[0].broken_ordered|not),verified:true}' > "$OUT/gate-summary.json"
echo "config-profiles gate passed: recall/precision trade-off monotonic across profiles, broken profile rejected"
