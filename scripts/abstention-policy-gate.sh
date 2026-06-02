#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/abstention-policy-gate.json}"; OUT="${2:-results/generated/abstention-policy}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.abstention-policy-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "abstention" "make abstention-policy-gate"; do grep -F "$phrase" docs/abstention-policy.md README.md > /dev/null; done
bash scripts/abstention-policy.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.abstention-policy/v1" and .meets_floor==true and .selective_accuracy==1 and .full_coverage_meets_floor==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.abstention-policy-gate-results/v1",coverage:$r[0].coverage,selective_accuracy:$r[0].selective_accuracy,meets_floor:$r[0].meets_floor,verified:true}' > "$OUT/gate-summary.json"
echo "abstention-policy gate passed: selective accuracy meets floor at achieved coverage, full coverage drops below floor"
