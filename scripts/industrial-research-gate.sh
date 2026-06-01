#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

GATES="${1:-examples/industrial-research-gates.json}"
OUT="${2:-results/generated/industrial-research-gate}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.industrial-research-gates/v1" and
  (.gates | length) > 0 and
  all(.gates[];
    (.id | length) > 0 and
    (.minimum_public_repos >= 4) and
    (.practical_fields | length) >= 5 and
    (.experiment_fields | length) >= 5 and
    (.alignment_claim | length) > 60
  )
' "$GATES" > /dev/null

bash scripts/four-repo-analysis-demo.sh "$OUT/matrix"

jq -e --slurpfile gates "$GATES" '
  ($gates[0].gates[0].minimum_public_repos) as $min |
  (.slices | length) >= $min and
  all(.slices[];
    (.performance.maintainer_review_burden > 0) and
    (.ranked_risks > 0) and
    (.linked_candidates > 0) and
    (.generated_artifacts > 0) and
    (.cache_hit == true or .cache_hit == false) and
    (.grep_only_comparison.ranked_risk_delta >= 0) and
    (.sql_only_comparison.ranked_risk_delta >= 0) and
    (.generation_comparison.fact_grounded_prompt_hash != .generation_comparison.without_facts_prompt_hash) and
    (.generation_comparison.fact_grounded_output_hash != .generation_comparison.without_facts_output_hash) and
    (.verification_comparison.deterministic_intervention_loops > 0) and
    (.sampled_adjudications.false_positive_samples > 0) and
    (.sampled_adjudications.false_negative_samples > 0)
  )
' "$OUT/matrix/slice-matrix.json" > /dev/null

jq -n \
  --slurpfile gates "$GATES" \
  --slurpfile matrix "$OUT/matrix/slice-matrix.json" \
  '{
    version: "patchline.industrial-research-gate-results/v1",
    gate: $gates[0].gates[0].id,
    public_repos: ($matrix[0].slices | map(.repo) | unique | length),
    practical_fields: $gates[0].gates[0].practical_fields,
    experiment_fields: $gates[0].gates[0].experiment_fields,
    alignment_claim: $gates[0].gates[0].alignment_claim,
    verified: true
  }' > "$OUT/summary.json"

echo "industrial-research gate passed: $(jq '.public_repos' "$OUT/summary.json") public repos have practical report fields and experiment fields"
