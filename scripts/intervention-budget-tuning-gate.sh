#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/intervention-budget-tuning-gate.json}"
OUT="${2:-results/generated/intervention-budget-tuning-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.intervention-budget-tuning-gate/v1" and (.dimensions|length)==4' "$SPEC" > /dev/null

for phrase in "budget tuning" "files" "lines" "tokens" "changes" "knee" "monotonic" "make intervention-budget-tuning-gate"; do
  grep -F "$phrase" docs/intervention-budget-tuning.md README.md > /dev/null
done

bash scripts/intervention-budget-tuning.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in costs.json study.jsonl intervention-budget-tuning.json intervention-budget-tuning.md README.md; do
  test -s "$OUT/$output"
done

minr="$(jq '.minimum_risks' "$SPEC")"

jq -e --argjson minr "$minr" '
  .version == "patchline.intervention-budget-tuning/v1" and
  .total_risks >= $minr and
  (.dimensions | length) == 4 and
  .all_monotonic == true and
  .all_zero_budget_empty == true and
  .all_full_budget_complete == true and
  .all_have_knee == true and
  .stable == true
' "$OUT/intervention-budget-tuning.json" > /dev/null

# Independently re-verify monotonicity of every coverage curve.
bad="$(jq '[.dimensions[] | .coverage_curve as $c | [range(1;($c|length)) | ($c[.].covered >= $c[.-1].covered)] | all] | map(select(.|not)) | length' "$OUT/intervention-budget-tuning.json")"
if [ "$bad" -ne 0 ]; then echo "found $bad non-monotonic curves"; exit 1; fi

# Declared dimensions must match the studied dimensions exactly.
spec_dims="$(jq -c '.dimensions | sort' "$SPEC")"
out_dims="$(jq -c '[.dimensions[].dimension] | sort' "$OUT/intervention-budget-tuning.json")"
if [ "$spec_dims" != "$out_dims" ]; then echo "dimension mismatch: $spec_dims vs $out_dims"; exit 1; fi

jq -n --slurpfile r "$OUT/intervention-budget-tuning.json" '{
  version: "patchline.intervention-budget-tuning-gate-results/v1",
  total_risks: $r[0].total_risks,
  dimensions: [$r[0].dimensions[] | {dimension, knee_pct, full_budget_covers}],
  all_monotonic: $r[0].all_monotonic,
  all_have_knee: $r[0].all_have_knee,
  verified: true
}' > "$OUT/gate-summary.json"

echo "intervention budget tuning gate passed: $(jq '.total_risks' "$OUT/gate-summary.json") risks, all dimensions monotonic with a knee"
