#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/ablation-study-gate.json}"
OUT="${2:-results/generated/ablation-study-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.ablation-study-gate/v1" and (.minimum_total_risks | numbers)' "$SPEC" > /dev/null

for phrase in "Ablation study" "score-factor" "Runtime-traces ablation" "Risk-budgets ablation" "make ablation-study-gate"; do
  grep -F "$phrase" docs/ablation-study.md README.md > /dev/null
done

bash scripts/ablation-study.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in ablation-study.json ablation-study.md README.md; do
  test -s "$OUT/$output"
done

min_axes="$(jq '.minimum_score_factor_axes' "$SPEC")"
min_risks="$(jq '.minimum_total_risks' "$SPEC")"

jq -e --argjson min_axes "$min_axes" --argjson min_risks "$min_risks" '
  .version == "patchline.ablation-study/v1" and
  .total_risks >= $min_risks and
  (.score_factor_axes | length) >= $min_axes and
  # Every score-factor axis must be non-degenerate: it affected risks and removed weight.
  all(.score_factor_axes[]; .affected_risks >= 1 and .total_weight_removed > 0) and
  # Provenance must be one of the ablated axes.
  (.score_factor_axes | map(.axis) | index("provenance-links") != null) and
  # Runtime ablation must be computed from real proof obligations.
  .runtime_traces.total_proof_summaries >= 1 and
  (.runtime_traces.unresolved_fraction >= 0 and .runtime_traces.unresolved_fraction <= 1) and
  # Budget curve must be monotonic non-decreasing in score capture.
  .risk_budgets.monotonic == true and
  (.risk_budgets.curve | length) >= 2 and
  # The largest budget must capture the most score.
  (.risk_budgets.curve | last.score_capture) >= (.risk_budgets.curve | first.score_capture) and
  (.ranking_by_impact | length) == (.score_factor_axes | length)
' "$OUT/ablation-study.json" > /dev/null

jq -n --slurpfile r "$OUT/ablation-study.json" '{
  version: "patchline.ablation-study-gate-results/v1",
  total_risks: $r[0].total_risks,
  top_impact_axis: $r[0].ranking_by_impact[0],
  runtime_unresolved_fraction: $r[0].runtime_traces.unresolved_fraction,
  largest_budget_score_capture: ($r[0].risk_budgets.curve | last.score_capture),
  verified: true
}' > "$OUT/gate-summary.json"

echo "ablation study gate passed: risks $(jq '.total_risks' "$OUT/gate-summary.json"), top impact $(jq -r '.top_impact_axis' "$OUT/gate-summary.json"), runtime unresolved $(jq '.runtime_unresolved_fraction' "$OUT/gate-summary.json"), max-budget capture $(jq '.largest_budget_score_capture' "$OUT/gate-summary.json")"
