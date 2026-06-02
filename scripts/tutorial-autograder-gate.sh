#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/tutorial-autograder-gate.json}"; OUT="${2:-results/generated/tutorial-autograder}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.tutorial-autograder-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "autograder" "make tutorial-autograder-gate"; do grep -F "$phrase" docs/tutorial-autograder.md README.md > /dev/null; done
bash scripts/tutorial-autograder.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.tutorial-autograder/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.tutorial-autograder-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "tutorial-autograder gate passed: every exercise gate-autograded, ungraded exercise rejected"
