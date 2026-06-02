#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/reviewer-action-model-gate.json}"; OUT="${2:-results/generated/reviewer-action-model}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.reviewer-action-model-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "predict" "make reviewer-action-model-gate"; do grep -F "$phrase" docs/reviewer-action-model.md README.md > /dev/null; done
bash scripts/reviewer-action-model.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.reviewer-action-model/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.reviewer-action-model-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "reviewer-action-model gate passed: predictions match observed actions, mispredicted case rejected"
