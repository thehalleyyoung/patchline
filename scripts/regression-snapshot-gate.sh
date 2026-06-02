#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/regression-snapshot-gate.json}"; OUT="${2:-results/generated/regression-snapshot}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.regression-snapshot-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "newly introduced" "make regression-snapshot-gate"; do grep -F "$phrase" docs/regression-snapshot.md README.md > /dev/null; done
bash scripts/regression-snapshot.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.regression-snapshot/v1" and .fails_on_new==true and .passes_without_new==true and .new_hazards==1' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.regression-snapshot-gate-results/v1",new_hazards:$r[0].new_hazards,fails_on_new:$r[0].fails_on_new,passes_without_new:$r[0].passes_without_new,verified:true}' > "$OUT/gate-summary.json"
echo "regression-snapshot gate passed: fails on newly introduced hazard, passes when none introduced"
