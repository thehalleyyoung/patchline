#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/office-hours-rotation-gate.json}"; OUT="${2:-results/generated/office-hours-rotation}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.office-hours-rotation-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "rotation" "make office-hours-rotation-gate"; do grep -F "$phrase" docs/office-hours-rotation.md README.md > /dev/null; done
bash scripts/office-hours-rotation.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.office-hours-rotation/v1" and .full_coverage==true and .no_conflict==true and .broken_full==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.office-hours-rotation-gate-results/v1",full_coverage:$r[0].full_coverage,no_conflict:$r[0].no_conflict,broken_rejected:($r[0].broken_full|not),verified:true}' > "$OUT/gate-summary.json"
echo "office-hours-rotation gate passed: full coverage with no conflicts, unstaffed schedule rejected"
