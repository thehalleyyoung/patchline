#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/architecture-layer-gate.json}"
OUT="${2:-results/generated/architecture-layer-gate}"
DOC="${3:-docs/architecture.md}"
rm -rf "$OUT"
mkdir -p "$OUT/cases" "$OUT/cache"

jq -e '
  .version == "patchline.architecture-layer-gate/v1" and
  .minimum_public_repos >= 4 and
  (.slices | length) >= .minimum_public_repos and
  (.required_layers | sort) == (["baseline","compare","deep-analysis","fetch","gate","intake","inventory","proposal"] | sort) and
  all(.slices[]; (.id | length) > 0 and (.repo | length) > 0 and (.ref | length) == 40 and (.subpath | length) > 0)
' "$SPEC" > /dev/null

for heading in \
  "## Fetch layer" \
  "## Inventory layer" \
  "## Intake layer" \
  "## Baseline layer" \
  "## Proposal layer" \
  "## Compare layer" \
  "## Deep-analysis layer" \
  "## Gate layer" \
  "## End-to-end artifact contract" \
  "## Trust boundaries" \
  "## Extension points"; do
  grep -F "$heading" "$DOC" > /dev/null
done

for term in \
  "source.json" \
  "inventory/inventory.json" \
  "intake/intake.json" \
  "baseline/baseline.json" \
  "proposal/proposal.json" \
  "compare/compare.json" \
  "analysis-bundle/summary.md" \
  "summary.sarif" \
  "patchline repo analyze" \
  "patchline repo offline" \
  "Generated files are analyzed as untrusted source inputs"; do
  grep -F "$term" "$DOC" > /dev/null
done

if grep -E '100_STEPS|100_steps|NEWEST_PLAN|NEW_PLAN' "$DOC" > /dev/null; then
  echo "architecture doc contains roadmap-only reference" >&2
  exit 1
fi

rows=()
while IFS=$'\t' read -r id repo ref subpath; do
  case_out="$OUT/cases/$id"
  mkdir -p "$case_out"
  go run ./cmd/patchline repo analyze \
    --github "$repo" \
    --ref "$ref" \
    --subpath "$subpath" \
    --download-dir "$OUT/cache" \
    --stages inventory,baseline,propose,compare,deep \
    --proposal-kind all \
    --budget files=4,lines=80,tokens=12000,changes=2 \
    --no-llm \
    --out "$case_out/analyze" \
    --json > "$case_out/stdout.json"

  test -s "$case_out/analyze/fetch/source.json"
  test -s "$case_out/analyze/inventory/inventory.json"
  test -s "$case_out/analyze/intake/summary.json"
  test -s "$case_out/analyze/baseline/baseline.json"
  test -s "$case_out/analyze/proposal/proposal.json"
  test -s "$case_out/analyze/compare/compare.json"
  test -s "$case_out/analyze/triage/triage.json"
  test -s "$case_out/analyze/analysis-bundle/source.json"
  test -s "$case_out/analyze/analysis-bundle/baseline.json"
  test -s "$case_out/analyze/analysis-bundle/compare.json"
  test -s "$case_out/analyze/analysis-bundle/summary.md"
  test -s "$case_out/analyze/analysis-bundle/summary.sarif"
  test -s "$case_out/analyze/commands.md"

  jq -e '
    .version == "patchline.repo-analyze/v1" and
    .stages == ["inventory","baseline","propose","compare","deep"] and
    (.outputs | has("fetch") and has("inventory") and has("intake") and has("baseline") and has("proposal") and has("compare") and has("triage") and has("analysis_bundle") and has("commands")) and
    .summary.files_scanned > 0 and
    .summary.facts > 0 and
    .summary.ranked_risks > 0 and
    .summary.ranking_explanations > 0 and
    .summary.provenance_slices > 0 and
    .summary.policy_checks > 0 and
    .summary.repair_proof_summaries > 0 and
    .summary.generated_files > 0 and
    .summary.intervention_loops > 0 and
    (.summary.baseline_hash | length) > 0 and
    (.summary.proposal_hash | length) > 0 and
    (.summary.compare_hash | length) > 0 and
    .deep_analysis.abstract_effects > 0 and
    .deep_analysis.symbolic_checks > 0 and
    .deep_analysis.temporal_windows > 0 and
    .deep_analysis.ablation_sensitive_risks > 0 and
    (.hash | length) > 0
  ' "$case_out/analyze/analyze.json" > /dev/null

  jq -e '
    .summary.problem_candidates >= 0 and
    .summary.cause_candidates >= 0 and
    .summary.repair_candidates >= 0
  ' "$case_out/analyze/intake/summary.json" > /dev/null

  jq -e '
    .summary.ranked_risks > 0 and
    .summary.provenance_slices > 0 and
    .summary.policy_checks > 0 and
    .summary.repair_proof_summaries > 0
  ' "$case_out/analyze/baseline/baseline.json" > /dev/null

  jq -e '
    (.generated_files | length) > 0 and
    (.deterministic_only == true) and
    (.scope_budget.raw | length) > 0
  ' "$case_out/analyze/proposal/proposal.json" > /dev/null

  jq -e '
    .summary.intervention_loops > 0 and
    (.intervention_loop.status | length) > 0
  ' "$case_out/analyze/compare/compare.json" > /dev/null

  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg subpath "$subpath" \
    --slurpfile analyze "$case_out/analyze/analyze.json" \
    '{
      id:$id,
      repo:$repo,
      subpath:$subpath,
      files_scanned:$analyze[0].summary.files_scanned,
      ranked_risks:$analyze[0].summary.ranked_risks,
      generated_files:$analyze[0].summary.generated_files,
      intervention_loops:$analyze[0].summary.intervention_loops,
      provenance_slices:$analyze[0].summary.provenance_slices,
      symbolic_checks:$analyze[0].deep_analysis.symbolic_checks,
      verified:true
    }' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.slices[] | [.id, .repo, .ref, .subpath] | @tsv' "$SPEC")

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile rows <(jq -s '.' "${rows[@]}") \
  '{
    version:"patchline.architecture-layer-gate-results/v1",
    claim:$spec[0].claim,
    document:"docs/architecture.md",
    slices:$rows[0],
    summary:{
      public_repos:($rows[0] | map(.repo) | unique | length),
      verified:($rows[0] | map(select(.verified == true)) | length),
      files_scanned:($rows[0] | map(.files_scanned) | add),
      ranked_risks:($rows[0] | map(.ranked_risks) | add),
      generated_files:($rows[0] | map(.generated_files) | add),
      intervention_loops:($rows[0] | map(.intervention_loops) | add),
      provenance_slices:($rows[0] | map(.provenance_slices) | add),
      symbolic_checks:($rows[0] | map(.symbolic_checks) | add)
    }
  }' > "$OUT/summary.json"

jq -e --slurpfile spec "$SPEC" '
  .version == "patchline.architecture-layer-gate-results/v1" and
  .summary.public_repos >= $spec[0].minimum_public_repos and
  .summary.verified >= $spec[0].minimum_public_repos and
  .summary.files_scanned > 0 and
  .summary.ranked_risks > 0 and
  .summary.generated_files > 0 and
  .summary.intervention_loops > 0 and
  .summary.provenance_slices > 0 and
  .summary.symbolic_checks > 0
' "$OUT/summary.json" > /dev/null

echo "architecture layer gate passed: $(jq '.summary.public_repos' "$OUT/summary.json") public repo slices verified against docs/architecture.md"
