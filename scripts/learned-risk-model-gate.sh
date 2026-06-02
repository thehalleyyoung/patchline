#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/learned-risk-model-gate.json}"; OUT="${2:-results/generated/learned-risk-model}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.learned-risk-model-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "held-out" "make learned-risk-model-gate"; do grep -F "$phrase" docs/learned-risk-model.md README.md > /dev/null; done
bash scripts/learned-risk-model.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.learned-risk-model/v1" and .held_out==true and .beats_baseline==true and .accuracy==1' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.learned-risk-model-gate-results/v1",accuracy:$r[0].accuracy,brier:$r[0].brier,held_out:$r[0].held_out,verified:true}' > "$OUT/gate-summary.json"
echo "learned-risk-model gate passed: learned model evaluated held-out and beats baseline"
