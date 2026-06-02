#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/grand-unified-evidence-index-gate.json}"; OUT="${2:-results/generated/grand-unified-evidence-index}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.grand-unified-evidence-index-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "novelty" "make grand-unified-evidence-index-gate"; do grep -F "$phrase" docs/grand-unified-evidence-index.md README.md > /dev/null; done
bash scripts/grand-unified-evidence-index.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.grand-unified-evidence-index/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.grand-unified-evidence-index-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "grand-unified-evidence-index gate passed: every pillar gate-backed, unbacked pillar rejected"
