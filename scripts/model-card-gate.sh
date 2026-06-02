#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/model-card-gate.json}"; OUT="${2:-results/generated/model-card}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.model-card-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "failure mode" "make model-card-gate"; do grep -F "$phrase" docs/model-card.md README.md > /dev/null; done
bash scripts/model-card.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.model-card/v1" and .complete==true and (.failure_modes>=1) and .incomplete_complete==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.model-card-gate-results/v1",complete:$r[0].complete,failure_modes:$r[0].failure_modes,incomplete_rejected:($r[0].incomplete_complete|not),verified:true}' > "$OUT/gate-summary.json"
echo "model-card gate passed: model card complete with failure modes and metrics, incomplete card rejected"
