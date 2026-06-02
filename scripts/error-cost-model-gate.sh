#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/error-cost-model-gate.json}"; OUT="${2:-results/generated/error-cost-model}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.error-cost-model-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "expected cost" "make error-cost-model-gate"; do grep -F "$phrase" docs/error-cost-model.md README.md > /dev/null; done
bash scripts/error-cost-model.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '
  .version == "patchline.error-cost-model/v1" and
  .cost_worst == "config_a" and .raw_worst == "config_b" and
  .ranking_flips == true and
  (.scored.config_a.expected_cost == 100) and (.scored.config_b.expected_cost == 23)
' "$OUT/cost.json" > /dev/null
jq -n --slurpfile r "$OUT/cost.json" '{version:"patchline.error-cost-model-gate-results/v1", scored:$r[0].scored, ranking_flips:$r[0].ranking_flips, verified:true}' > "$OUT/gate-summary.json"
echo "error-cost-model gate passed: severity-weighted cost flips the naive raw-miss ranking"
