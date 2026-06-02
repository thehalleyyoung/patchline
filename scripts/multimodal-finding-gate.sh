#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/multimodal-finding-gate.json}"; OUT="${2:-results/generated/multimodal-finding}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.multimodal-finding-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "multimodal" "make multimodal-finding-gate"; do grep -F "$phrase" docs/multimodal-finding.md README.md > /dev/null; done
bash scripts/multimodal-finding.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.multimodal-finding/v1" and .all_consistent==true and .inconsistent_consistent==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.multimodal-finding-gate-results/v1",consistent:$r[0].consistent,all_consistent:$r[0].all_consistent,inconsistent_flagged:($r[0].inconsistent_consistent|not),verified:true}' > "$OUT/gate-summary.json"
echo "multimodal-finding gate passed: findings cross-modally consistent, inconsistent finding flagged"
