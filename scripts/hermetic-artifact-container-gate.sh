#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/hermetic-artifact-container-gate.json}"; OUT="${2:-results/generated/hermetic-artifact-container}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.hermetic-artifact-container-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "Artifacts-Available" "make hermetic-artifact-container-gate"; do grep -F "$phrase" docs/hermetic-artifact-container.md README.md > /dev/null; done
bash scripts/hermetic-artifact-container.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.hermetic-artifact-container/v1" and .all_satisfied==true and .hermetic==true and .leaky_hermetic==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.hermetic-artifact-container-gate-results/v1",all_satisfied:$r[0].all_satisfied,hermetic:$r[0].hermetic,leaky_rejected:($r[0].leaky_hermetic|not),verified:true}' > "$OUT/gate-summary.json"
echo "hermetic-artifact-container gate passed: checklist satisfied under hermetic conditions, network-requiring container rejected"
