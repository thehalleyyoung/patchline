#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/rebuttal-evidence-pack-gate.json}"; OUT="${2:-results/generated/rebuttal-evidence-pack}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.rebuttal-evidence-pack-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "reproducible answer" "make rebuttal-evidence-pack-gate"; do grep -F "$phrase" docs/rebuttal-evidence-pack.md README.md > /dev/null; done
bash scripts/rebuttal-evidence-pack.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.rebuttal-evidence-pack/v1" and .all_answered==true and .unanswered_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.rebuttal-evidence-pack-gate-results/v1",answered:$r[0].answered,all_answered:$r[0].all_answered,unanswered_rejected:($r[0].unanswered_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "rebuttal-evidence-pack gate passed: every reviewer question has a reproducible answer, unanswered question rejected"
