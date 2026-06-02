#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/performance-budget-gate.json}"
OUT="${2:-results/generated/performance-budget-gate}"
DOC="${3:-docs/performance-budgets.md}"
rm -rf "$OUT"
mkdir -p "$OUT/cases" "$OUT/cache"

jq -e '
  .version == "patchline.performance-budget-gate/v1" and
  (.claim | length) > 40 and
  .budgets.large_repo_seconds > 0 and
  .budgets.monorepo_seconds > 0 and
  .budgets.generated_bundle_seconds > 0 and
  .budgets.generated_bundle_bytes > 0 and
  .budgets.matrix_case_seconds > 0 and
  .budgets.matrix_total_seconds > 0 and
  (.matrix.slices | length) >= .matrix.minimum_public_repos and
  all([.large_repo, .monorepo, .generated_bundle][]; (.id | length) > 0 and (.repo | length) > 0 and (.ref | length) == 40 and (.subpath | length) > 0) and
  all(.matrix.slices[]; (.id | length) > 0 and (.repo | length) > 0 and (.ref | length) == 40 and (.subpath | length) > 0)
' "$SPEC" > /dev/null

for term in \
  "Large repository slice" \
  "Monorepo slice" \
  "Generated bundle" \
  "Four-repo matrix" \
  "make performance-budget-gate"; do
  grep -F "$term" "$DOC" > /dev/null
done

run_analyze_case() {
  local id="$1"
  local repo="$2"
  local ref="$3"
  local subpath="$4"
  local budget="$5"
  local kind="$6"
  local extra_flags="${7:-}"
  local case_out="$OUT/cases/$id"
  mkdir -p "$case_out"
  local started
  started="$(date +%s)"
  # shellcheck disable=SC2086
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
    --json \
    $extra_flags > "$case_out/stdout.json"
  local finished elapsed
  finished="$(date +%s)"
  elapsed=$((finished - started))

  jq -e --argjson budget "$budget" '
    .version == "patchline.repo-analyze/v1" and
    .summary.files_scanned > 0 and
    .summary.ranked_risks > 0 and
    .summary.generated_files > 0 and
    .summary.intervention_loops > 0 and
    (.hash | length) > 0
  ' "$case_out/analyze/analyze.json" > /dev/null

  test "$elapsed" -le "$budget"
  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg subpath "$subpath" \
    --arg kind "$kind" \
    --argjson budget "$budget" \
    --argjson elapsed "$elapsed" \
    --slurpfile analyze "$case_out/analyze/analyze.json" \
    '{
      id:$id,
      kind:$kind,
      repo:$repo,
      subpath:$subpath,
      elapsed_seconds:$elapsed,
      budget_seconds:$budget,
      files_scanned:$analyze[0].summary.files_scanned,
      ranked_risks:$analyze[0].summary.ranked_risks,
      generated_files:$analyze[0].summary.generated_files,
      intervention_loops:$analyze[0].summary.intervention_loops,
      verified:($elapsed <= $budget)
    }' > "$case_out/row.json"
}

read -r large_id large_repo large_ref large_subpath large_budget < <(
  jq -r '[.large_repo.id, .large_repo.repo, .large_repo.ref, .large_repo.subpath, .budgets.large_repo_seconds] | @tsv' "$SPEC"
)
run_analyze_case "$large_id" "$large_repo" "$large_ref" "$large_subpath" "$large_budget" "large-repo"

read -r mono_id mono_repo mono_ref mono_subpath mono_budget < <(
  jq -r '[.monorepo.id, .monorepo.repo, .monorepo.ref, .monorepo.subpath, .budgets.monorepo_seconds] | @tsv' "$SPEC"
)
run_analyze_case "$mono_id" "$mono_repo" "$mono_ref" "$mono_subpath" "$mono_budget" "monorepo"

read -r bundle_id bundle_repo bundle_ref bundle_subpath bundle_budget bundle_byte_budget < <(
  jq -r '[.generated_bundle.id, .generated_bundle.repo, .generated_bundle.ref, .generated_bundle.subpath, .budgets.generated_bundle_seconds, .budgets.generated_bundle_bytes] | @tsv' "$SPEC"
)
run_analyze_case "$bundle_id" "$bundle_repo" "$bundle_ref" "$bundle_subpath" "$bundle_budget" "generated-bundle" "--redact --ci"
bundle_dir="$OUT/cases/$bundle_id/analyze/analysis-bundle"
bundle_bytes="$(find "$bundle_dir" -type f -print0 | xargs -0 stat -f '%z' | awk '{sum += $1} END {print sum + 0}')"
test "$bundle_bytes" -le "$bundle_byte_budget"
jq --argjson bundle_bytes "$bundle_bytes" --argjson bundle_byte_budget "$bundle_byte_budget" \
  '. + {bundle_bytes:$bundle_bytes, bundle_byte_budget:$bundle_byte_budget, bundle_within_budget:($bundle_bytes <= $bundle_byte_budget)}' \
  "$OUT/cases/$bundle_id/row.json" > "$OUT/cases/$bundle_id/row.tmp"
mv "$OUT/cases/$bundle_id/row.tmp" "$OUT/cases/$bundle_id/row.json"
test -s "$bundle_dir/summary.md"
test -s "$bundle_dir/summary.sarif"
test -s "$OUT/cases/$bundle_id/analyze/ci/summary.md"

matrix_rows=()
matrix_started="$(date +%s)"
while IFS=$'\t' read -r id repo ref subpath case_budget; do
  run_analyze_case "$id" "$repo" "$ref" "$subpath" "$case_budget" "four-repo-matrix"
  matrix_rows+=("$OUT/cases/$id/row.json")
done < <(jq -r '.budgets.matrix_case_seconds as $budget | .matrix.slices[] | [.id, .repo, .ref, .subpath, $budget] | @tsv' "$SPEC")
matrix_finished="$(date +%s)"
matrix_elapsed=$((matrix_finished - matrix_started))
matrix_budget="$(jq -r '.budgets.matrix_total_seconds' "$SPEC")"
test "$matrix_elapsed" -le "$matrix_budget"

all_rows=(
  "$OUT/cases/$large_id/row.json"
  "$OUT/cases/$mono_id/row.json"
  "$OUT/cases/$bundle_id/row.json"
  "${matrix_rows[@]}"
)

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile rows <(jq -s '.' "${all_rows[@]}") \
  --argjson matrix_elapsed "$matrix_elapsed" \
  --argjson matrix_budget "$matrix_budget" \
  '{
    version:"patchline.performance-budget-gate-results/v1",
    claim:$spec[0].claim,
    budgets:$spec[0].budgets,
    runs:$rows[0],
    summary:{
      runs:($rows[0] | length),
      verified:($rows[0] | map(select(.verified == true)) | length),
      public_repos:($rows[0] | map(.repo) | unique | length),
      total_elapsed_seconds:($rows[0] | map(.elapsed_seconds) | add),
      max_elapsed_seconds:($rows[0] | map(.elapsed_seconds) | max),
      files_scanned:($rows[0] | map(.files_scanned) | add),
      ranked_risks:($rows[0] | map(.ranked_risks) | add),
      generated_files:($rows[0] | map(.generated_files) | add),
      matrix_elapsed_seconds:$matrix_elapsed,
      matrix_budget_seconds:$matrix_budget,
      matrix_within_budget:($matrix_elapsed <= $matrix_budget),
      generated_bundle_bytes:($rows[0] | map(.bundle_bytes // 0) | add)
    }
  }' > "$OUT/summary.json"

jq -e --slurpfile spec "$SPEC" '
  .version == "patchline.performance-budget-gate-results/v1" and
  .summary.verified == .summary.runs and
  .summary.public_repos >= $spec[0].matrix.minimum_public_repos and
  .summary.files_scanned > 0 and
  .summary.ranked_risks > 0 and
  .summary.generated_files > 0 and
  .summary.matrix_within_budget == true and
  all(.runs[]; .elapsed_seconds <= .budget_seconds and .verified == true)
' "$OUT/summary.json" > /dev/null

echo "performance budget gate passed: $(jq '.summary.runs' "$OUT/summary.json") runs, max $(jq '.summary.max_elapsed_seconds' "$OUT/summary.json")s, matrix $(jq '.summary.matrix_elapsed_seconds' "$OUT/summary.json")s"
