#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/transfer-learning-study-gate.json}"; OUT="${2:-results/generated/transfer-learning-study}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.transfer-learning-study-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "zero-shot" "make transfer-learning-study-gate"; do grep -F "$phrase" docs/transfer-learning-study.md README.md > /dev/null; done
bash scripts/transfer-learning-study.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.transfer-learning-study/v1" and .disjoint==true and .clears_threshold==true and .leaked_disjoint==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.transfer-learning-study-gate-results/v1",zero_shot_accuracy:$r[0].zero_shot_accuracy,disjoint:$r[0].disjoint,leak_rejected:($r[0].leaked_disjoint|not),verified:true}' > "$OUT/gate-summary.json"
echo "transfer-learning-study gate passed: ecosystems disjoint and zero-shot clears threshold, leaked split rejected"
