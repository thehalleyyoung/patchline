#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/cutover-protocol-model-gate.json}"; OUT="${2:-results/generated/cutover-protocol-model}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.cutover-protocol-model-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "safety invariant" "make cutover-protocol-model-gate"; do grep -F "$phrase" docs/cutover-protocol-model.md README.md > /dev/null; done
bash scripts/cutover-protocol-model.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.cutover-protocol-model/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.cutover-protocol-model-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "cutover-protocol-model gate passed: safety invariant holds in every modeled state, violating state rejected"
