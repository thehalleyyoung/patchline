#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/continual-learning-eval-gate.json}"; OUT="${2:-results/generated/continual-learning-eval}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.continual-learning-eval-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "forgetting" "make continual-learning-eval-gate"; do grep -F "$phrase" docs/continual-learning-eval.md README.md > /dev/null; done
bash scripts/continual-learning-eval.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.continual-learning-eval/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.continual-learning-eval-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "continual-learning-eval gate passed: no release forgets old tasks, forgetting release rejected"
