#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/stage-ablation-gate.json}"; OUT="${2:-results/generated/stage-ablation}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.stage-ablation-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "marginal" "make stage-ablation-gate"; do grep -F "$phrase" docs/stage-ablation.md README.md > /dev/null; done
bash scripts/stage-ablation.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '
  .version == "patchline.stage-ablation/v1" and
  (.marginal.taint > 0) and (.marginal.backfill > 0) and
  (.marginal.redundant_duplicate == 0) and
  (.load_bearing | index("taint")) and (.load_bearing | index("backfill")) and
  (.redundant == ["redundant_duplicate"])
' "$OUT/ablation.json" > /dev/null
jq -n --slurpfile r "$OUT/ablation.json" '{version:"patchline.stage-ablation-gate-results/v1", marginal:$r[0].marginal, load_bearing:$r[0].load_bearing, redundant:$r[0].redundant, verified:true}' > "$OUT/gate-summary.json"
echo "stage-ablation gate passed: load-bearing stages have positive marginal, redundant stage contributes zero"
