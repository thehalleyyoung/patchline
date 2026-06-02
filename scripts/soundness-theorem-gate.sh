#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/soundness-theorem-gate.json}"; OUT="${2:-results/generated/soundness-theorem}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.soundness-theorem-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "soundness" "make soundness-theorem-gate"; do grep -F "$phrase" docs/soundness-theorem.md README.md > /dev/null; done
bash scripts/soundness-theorem.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.soundness-theorem/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.soundness-theorem-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "soundness-theorem gate passed: every hazard class soundly proven, unproven class rejected"
