#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

GATES="${1:-examples/development-cycle-gates.json}"
OUT="${2:-results/generated/development-cycle-gate}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.development-cycle-gates/v1" and
  (.gates | length) > 0 and
  all(.gates[];
    (.id | length) > 0 and
    (.minimum_public_repos >= 4) and
    (.previous_cycle | length) > 20 and
    (.current_cycle | length) > 20 and
    (.required_improvements | length) >= 5 and
    (.cycle_claim | length) > 60
  )
' "$GATES" > /dev/null

bash scripts/four-repo-analysis-demo.sh "$OUT/capstone"

jq --slurpfile gates "$GATES" '{
  version: "patchline.development-cycle-gate-results/v1",
  gate: $gates[0].gates[0].id,
  previous_cycle: $gates[0].gates[0].previous_cycle,
  current_cycle: $gates[0].gates[0].current_cycle,
  cycle_claim: $gates[0].gates[0].cycle_claim,
  public_repos: (.cases | map(.repo) | unique | length),
  cases: [.cases[] | {
    label,
    repo,
    subpath,
    baseline_risks: .baseline.risks,
    previous_generated_artifacts: 0,
    previous_covered_risks: 0,
    current_generated_artifacts: .proposal.files,
    current_covered_risks: .compare.risks_with_coverage,
    deterministic_intervention_loops: .compare.intervention_loops,
    deterministic_failures: .compare.checks_failed,
    deep_reanalysis_dimensions: ([
      .baseline.provenance_slices,
      .baseline.datalog_rows,
      .baseline.abstract_effects,
      .baseline.symbolic_checks,
      .baseline.temporal_windows,
      .baseline.recurrences,
      .baseline.policy_checks,
      .baseline.repair_proof_summaries,
      .baseline.ranking_explanations
    ] | map(select(. > 0)) | length),
    generated_artifact_gain: .proposal.files,
    covered_risk_gain: .compare.risks_with_coverage,
    verified: (
      .baseline.risks > 0 and
      .proposal.files > 0 and
      .compare.risks_with_coverage > 0 and
      .compare.intervention_loops > 0 and
      .compare.checks_failed == 0 and
      ([
        .baseline.provenance_slices,
        .baseline.datalog_rows,
        .baseline.abstract_effects,
        .baseline.symbolic_checks,
        .baseline.temporal_windows,
        .baseline.recurrences,
        .baseline.policy_checks,
        .baseline.repair_proof_summaries,
        .baseline.ranking_explanations
      ] | map(select(. > 0)) | length) >= 5
    )
  }]
}' "$OUT/capstone/summary.json" > "$OUT/summary.json"

jq -e '.public_repos >= 4 and all(.cases[]; .verified == true and .generated_artifact_gain > 0 and .covered_risk_gain > 0)' "$OUT/summary.json" > /dev/null
echo "development-cycle gate passed: $(jq '.public_repos' "$OUT/summary.json") public repos show generated intervention gains followed by deterministic deep re-analysis"
