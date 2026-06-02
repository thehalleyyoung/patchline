#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/minimal-repro-generator-gate.json}"; OUT="${2:-results/generated/minimal-repro-generator}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.minimal-repro-generator-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "minimal reproduction" "make minimal-repro-generator-gate"; do grep -F "$phrase" docs/minimal-repro-generator.md README.md > /dev/null; done
bash scripts/minimal-repro-generator.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.minimal-repro-generator/v1" and .smaller==true and .verdict_preserved==true and .minimal==true and .over_reduced_preserved==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.minimal-repro-generator-gate-results/v1",reduced_size:$r[0].reduced_size,verdict_preserved:$r[0].verdict_preserved,over_reduction_rejected:($r[0].over_reduced_preserved|not),verified:true}' > "$OUT/gate-summary.json"
echo "minimal-repro-generator gate passed: reproduction reduced and verdict-preserving, over-reduction rejected"
