#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/gold-subset-gate.json}"; OUT="${2:-results/generated/gold-subset}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.gold-subset-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "gold" "make gold-subset-gate"; do grep -F "$phrase" docs/gold-subset.md README.md > /dev/null; done
bash scripts/gold-subset.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '
  .version == "patchline.gold-subset/v1" and
  (.gold_ids == ["m1","m2","m4","m5"]) and
  (.excluded == ["m3"]) and
  (.agreement_rate == 0.8) and
  ((.gold[] | select(.id=="m1") | .label) == "hazard")
' "$OUT/gold.json" > /dev/null
jq -n --slurpfile r "$OUT/gold.json" '{version:"patchline.gold-subset-gate-results/v1", gold_ids:$r[0].gold_ids, excluded:$r[0].excluded, agreement_rate:$r[0].agreement_rate, verified:true}' > "$OUT/gate-summary.json"
echo "gold-subset gate passed: agreed items form gold set, disagreement excluded, agreement rate correct"
