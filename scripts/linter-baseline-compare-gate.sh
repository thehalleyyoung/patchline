#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/linter-baseline-compare-gate.json}"; OUT="${2:-results/generated/linter-baseline-compare}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.linter-baseline-compare-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "linter" "make linter-baseline-compare-gate"; do grep -F "$phrase" docs/linter-baseline-compare.md README.md > /dev/null; done
bash scripts/linter-baseline-compare.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '
  .version == "patchline.linter-baseline-compare/v1" and
  .matched == true and .patchline_best == true and
  .recall.patchline == 1 and (.recall.patchline > .recall.linter_a) and
  .mismatched_rejected == true
' "$OUT/compare.json" > /dev/null
jq -n --slurpfile r "$OUT/compare.json" '{version:"patchline.linter-baseline-compare-gate-results/v1", recall:$r[0].recall, patchline_best:$r[0].patchline_best, mismatch_rejected:$r[0].mismatched_rejected, verified:true}' > "$OUT/gate-summary.json"
echo "linter-baseline-compare gate passed: matched inputs, Patchline recall dominates, mismatch rejected"
