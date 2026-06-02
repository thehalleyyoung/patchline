#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/adversarial-training-loop-gate.json}"; OUT="${2:-results/generated/adversarial-training-loop}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.adversarial-training-loop-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "hardening" "make adversarial-training-loop-gate"; do grep -F "$phrase" docs/adversarial-training-loop.md README.md > /dev/null; done
bash scripts/adversarial-training-loop.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.adversarial-training-loop/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.adversarial-training-loop-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "adversarial-training-loop gate passed: every round hardens the model, robustness regression rejected"
