#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/meta-gate-predictor-gate.json}"; OUT="${2:-results/generated/meta-gate-predictor}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.meta-gate-predictor-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "predict" "make meta-gate-predictor-gate"; do grep -F "$phrase" docs/meta-gate-predictor.md README.md > /dev/null; done
bash scripts/meta-gate-predictor.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.meta-gate-predictor/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.meta-gate-predictor-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "meta-gate-predictor gate passed: predictions match firing gate, mispredicted case rejected"
