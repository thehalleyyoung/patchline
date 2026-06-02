#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

QUESTIONS="${1:-examples/research-questions.json}"
OUT="${2:-results/generated/research-question-gate}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.research-questions/v1" and
  (.questions | length) >= 5 and
  ([.questions[].area] | index("repo-native fact extraction")) and
  ([.questions[].area] | index("risk ranking")) and
  ([.questions[].area] | index("evidence linking")) and
  ([.questions[].area] | index("generated-intervention safety")) and
  ([.questions[].area] | index("before/after re-analysis")) and
  all(.questions[]; (.primary_metrics | length) >= 3 and (.real_repo_requirement | contains("four pinned public slices")))
' "$QUESTIONS" > /dev/null

rows=()
while IFS=$'\t' read -r id repo ref subpath; do
  case_out="$OUT/$id"
  mkdir -p "$case_out"
  start_seconds="$(date +%s)"
  go run ./cmd/patchline repo analyze --github "$repo" --ref "$ref" --subpath "$subpath" --stages inventory,baseline,propose,compare --no-llm --out "$case_out/analysis" --json > "$case_out/analyze.json"
  runtime_seconds="$(($(date +%s) - start_seconds))"
  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg subpath "$subpath" \
    --argjson runtime_seconds "$runtime_seconds" \
    --slurpfile analyze "$case_out/analyze.json" \
    --slurpfile baseline "$case_out/analysis/baseline/baseline.json" \
    --slurpfile compare "$case_out/analysis/compare/compare.json" \
    '{
      id: $id,
      repo: $repo,
      subpath: $subpath,
      facts: $analyze[0].summary.facts,
      files_scanned: $analyze[0].summary.files_scanned,
      ranked_risks: $analyze[0].summary.ranked_risks,
      ranking_explanations: $analyze[0].summary.ranking_explanations,
      stable_risk_ids: ([ $baseline[0].risks[] | select((.stable_id // "") | startswith("stable-risk:")) ] | length),
      evidence_links: $baseline[0].summary.evidence_links,
      provenance_slices: $analyze[0].summary.provenance_slices,
      generated_files: $analyze[0].summary.generated_files,
      patchline_checks_passed: $compare[0].summary.patchline_checks_passed,
      patchline_checks_failed: $compare[0].summary.patchline_checks_failed,
      targeted_risks: $compare[0].summary.targeted_risks,
      warnings: $compare[0].summary.warnings,
      runtime_seconds: $runtime_seconds,
      review_burden_items: (($compare[0].summary.targeted_risks // 0) + ($analyze[0].summary.generated_files // 0) + ($compare[0].summary.warnings // 0) + ($compare[0].summary.patchline_checks_failed // 0)),
      compare_hash: $analyze[0].summary.compare_hash,
      intervention_loops: $analyze[0].summary.intervention_loops,
      verified: true
    }' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.slices[] | [.id, .repo, .ref, .subpath] | @tsv' examples/real-repo-slices.json)

jq -n \
  --slurpfile questions "$QUESTIONS" \
  --slurpfile rows <(jq -s '.' "${rows[@]}") \
  '{
    version:"patchline.research-question-gate-results/v1",
    questions:$questions[0].questions,
    slices:$rows[0],
    rq_coverage:{
      fact_extraction: all($rows[0][]; .facts > 0 and .files_scanned > 0),
      risk_ranking: all($rows[0][]; .ranked_risks > 0 and .stable_risk_ids > 0 and .ranking_explanations > 0),
      evidence_linking: all($rows[0][]; .evidence_links > 0 and .provenance_slices > 0),
      generated_safety: all($rows[0][]; .generated_files > 0 and .patchline_checks_failed == 0),
      before_after: all($rows[0][]; (.compare_hash | length) > 0 and .intervention_loops > 0)
    }
  }' > "$OUT/summary.json"

jq -e '
  (.questions | length) >= 5 and
  (.slices | length) >= 4 and
  all(.rq_coverage[]; . == true)
' "$OUT/summary.json" > /dev/null
echo "research-question gate passed: $(jq '.questions | length' "$OUT/summary.json") research questions operationalized on $(jq '.slices | length' "$OUT/summary.json") public slices"
