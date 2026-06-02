#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/llm-judge-harness-gate.json}"; OUT="${2:-results/generated/llm-judge-harness}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.llm-judge-harness-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "inter-rater" "make llm-judge-harness-gate"; do grep -F "$phrase" docs/llm-judge-harness.md README.md > /dev/null; done
bash scripts/llm-judge-harness.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.llm-judge-harness/v1" and .reliable==true and .agreement==1 and .unreliable_reliable==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.llm-judge-harness-gate-results/v1",agreement:$r[0].agreement,reliable:$r[0].reliable,unreliable_flagged:($r[0].unreliable_reliable|not),verified:true}' > "$OUT/gate-summary.json"
echo "llm-judge-harness gate passed: judges agree above threshold, chance-level pair flagged unreliable"
