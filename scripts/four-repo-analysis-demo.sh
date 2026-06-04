#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

OUT="${1:-results/generated/four-repo-analysis-demo}"
TIME_MODE="none"
if /usr/bin/time -l true >/dev/null 2>&1; then
  TIME_MODE="darwin"
elif /usr/bin/time -v true >/dev/null 2>&1; then
  TIME_MODE="gnu"
fi

measure_json_command() {
  local metrics_json="$1"
  local output_json="$2"
  shift 2
  local started ended rss raw
  started="$(date +%s)"
  if [[ "$TIME_MODE" == "darwin" ]]; then
    /usr/bin/time -l "$@" > "$output_json" 2> "$metrics_json.time"
  elif [[ "$TIME_MODE" == "gnu" ]]; then
    /usr/bin/time -v "$@" > "$output_json" 2> "$metrics_json.time"
  else
    "$@" > "$output_json"
    : > "$metrics_json.time"
  fi
  ended="$(date +%s)"
  raw="$(awk '/maximum resident set size/ {print $1} /Maximum resident set size/ {print $6}' "$metrics_json.time" | tail -n 1)"
  if [[ -z "$raw" ]]; then
    rss=0
  elif [[ "$TIME_MODE" == "darwin" ]]; then
    rss=$(( (raw + 1023) / 1024 ))
  else
    rss="$raw"
  fi
  jq -n \
    --arg mode "$TIME_MODE" \
    --argjson runtime "$((ended - started))" \
    --argjson max_rss_kb "$rss" \
    '{runtime_seconds: $runtime, max_rss_kb: $max_rss_kb, memory_sampler: $mode}' > "$metrics_json"
}

file_size_bytes() {
  local path="$1"
  if [[ -z "$path" || ! -f "$path" ]]; then
    echo 0
  elif stat -f%z "$path" >/dev/null 2>&1; then
    stat -f%z "$path"
  else
    stat -c%s "$path"
  fi
}

if [[ -n "${PATCHLINE_FOUR_REPO_CASES:-}" ]]; then
  CASES="$PATCHLINE_FOUR_REPO_CASES"
else
  CASES="$(jq -r '.slices[] | [.label, .repo, .ref, .subpath, .ecosystem, .migration_framework, .repo_size_class, (.available_evidence_types | join(","))] | @tsv' examples/real-repo-slices.json)"
fi

rm -rf "$OUT"
mkdir -p "$OUT/cases"

rows=()
while IFS=$'\t|' read -r label repo ref subpath ecosystem migration_framework repo_size_class evidence_types; do
  [[ -z "${label// }" ]] && continue
  if [[ -z "${subpath:-}" && -n "${ref:-}" && "$ref" != [0-9a-f][0-9a-f][0-9a-f][0-9a-f]* ]]; then
    subpath="$ref"
    ref=""
  fi
  ecosystem="${ecosystem:-unknown}"
  migration_framework="${migration_framework:-unknown}"
  repo_size_class="${repo_size_class:-unknown}"
  evidence_types="${evidence_types:-unknown}"
  slug="$(printf '%s-%s' "$repo" "$subpath" | tr '/[:space:]' '--' | tr -cd '[:alnum:]_.-')"
  case_out="$OUT/cases/$slug"

  echo "==> $label ($repo:$subpath)"
  PATCHLINE_REPO_DEMO_REPO="$repo" \
    PATCHLINE_REPO_DEMO_REF="$ref" \
    PATCHLINE_REPO_DEMO_SUBPATH="$subpath" \
    bash scripts/repo-analysis-demo.sh "$case_out"
  go run ./cmd/patchline repo propose --from-report "$case_out/baseline" --proposal-kind tests --budget files=1,lines=120,tokens=4000,changes=3 --llm-command "bash scripts/llm-command-smoke.sh" --out "$case_out/llm-proposal" --json > "$case_out/llm-proposal.json"
  go run ./cmd/patchline repo compare --before "$case_out/baseline" --after "$case_out/llm-proposal" --out "$case_out/llm-compare" --json > "$case_out/llm-compare.json"
  go run ./cmd/patchline repo propose --from-report "$case_out/baseline" --proposal-kind tests --budget files=1,lines=120,tokens=4000,changes=3 --llm-command "bash scripts/llm-command-smoke.sh" --prompt-without-facts --out "$case_out/no-facts-proposal" --json > "$case_out/no-facts-proposal.json"
  go run ./cmd/patchline repo compare --before "$case_out/baseline" --after "$case_out/no-facts-proposal" --out "$case_out/no-facts-compare" --json > "$case_out/no-facts-compare.json"
  go run ./cmd/patchline repo fetch "$repo" --ref "$ref" --subpath "$subpath" --out "$case_out/cache-proof" --json > "$case_out/cache-proof.json"
  archive_size_bytes="$(file_size_bytes "$(jq -r '.source.cache_path // ""' "$case_out/cache-proof.json")")"
  measure_json_command "$case_out/analyze-metrics.json" "$case_out/analyze.json" go run ./cmd/patchline repo analyze --github "$repo" --ref "$ref" --subpath "$subpath" --stages inventory,baseline,propose,compare,deep --proposal-kind all --budget files=15,lines=120,tokens=50000,changes=3 --ci --redact --no-llm --out "$case_out/analyze" --json
  jq -e '.input | startswith("[redacted:")' "$case_out/analyze/analysis-bundle/source.json" > /dev/null
  test -s "$case_out/analyze/ci/summary.md"
  grep -q "upload-sarif" "$case_out/analyze/ci/github-actions-upload.yml"
  grep -q "upload-artifact" "$case_out/analyze/ci/github-actions-upload.yml"
  test -s "$case_out/analyze/commands.md"
  grep -q "repo analyze" "$case_out/analyze/commands.md"
  grep -q "repo compare" "$case_out/analyze/commands.md"
  go run ./cmd/patchline repo analyze --github "$repo" --ref "$ref" --subpath "$subpath" --stages inventory,baseline,propose,compare,deep --proposal-kind all --budget files=15,lines=120,tokens=50000,changes=3 --ci --redact --no-llm --resume --out "$case_out/analyze" --json > "$case_out/analyze-resume.json"
  bundle_files="$(find "$case_out/analyze/analysis-bundle" -maxdepth 1 -type f | wc -l | tr -d ' ')"
  for required in source.json facts.jsonl baseline.json proposal.patch compare.json summary.md summary.sarif; do
    test -s "$case_out/analyze/analysis-bundle/$required"
  done
  go run ./cmd/patchline repo fetch "$repo" --ref "$ref" --subpath ".github/workflows" --out "$case_out/structured-fetch" --json > "$case_out/structured-fetch.json"
  structured_root="$(jq -r '.source.scanned_root' "$case_out/structured-fetch.json")"
  go run ./cmd/patchline repo inventory "$structured_root" --out "$case_out/structured-inventory" --json > "$case_out/structured-inventory.json"

  jq \
    --arg label "$label" \
    --arg repo "$repo" \
    --arg ref "$ref" \
    --arg subpath "$subpath" \
    --arg ecosystem "$ecosystem" \
    --arg migration_framework "$migration_framework" \
    --arg repo_size_class "$repo_size_class" \
    --arg evidence_types "$evidence_types" \
    --argjson archive_size_bytes "$archive_size_bytes" \
    --argjson bundle_files "$bundle_files" \
    --slurpfile llm "$case_out/llm-proposal.json" \
    --slurpfile llmcompare "$case_out/llm-compare.json" \
    --slurpfile nofacts "$case_out/no-facts-proposal.json" \
    --slurpfile nofactscompare "$case_out/no-facts-compare.json" \
    --slurpfile cache "$case_out/cache-proof.json" \
    --slurpfile metrics "$case_out/analyze-metrics.json" \
    --slurpfile analyze "$case_out/analyze.json" \
    --slurpfile resume "$case_out/analyze-resume.json" \
    --slurpfile structured "$case_out/structured-inventory.json" \
    --slurpfile adjudications examples/real-repo-adjudications.json \
    '. as $summary | ($adjudications[0].adjudications | map(select(.repo == $repo and .subpath == $subpath))) as $notes | $summary + {label: $label, repo: $repo, ref: $ref, subpath: $subpath, ecosystem: $ecosystem, migration_framework: $migration_framework, repo_size_class: $repo_size_class, available_evidence_types: ($evidence_types | split(",") | map(select(length > 0))), case_dir: "'"$case_out"'", sampled_adjudications: {notes: $notes, false_positive_samples: ($notes | map(select(.sample_type == "false_positive")) | length), false_negative_samples: ($notes | map(select(.sample_type == "false_negative")) | length)}, performance: {analyze_runtime_seconds: $metrics[0].runtime_seconds, analyze_max_rss_kb: $metrics[0].max_rss_kb, memory_sampler: $metrics[0].memory_sampler, download_size_bytes: $archive_size_bytes, cache_hit: $cache[0].source.cache_hit, maintainer_review_burden: (($summary.baseline.policy_checks // 0) + ($summary.baseline.repair_proof_summaries // 0) + ($summary.proposal.files // 0) + ($notes | length))}, llm_command_proof: {generator: $llm[0].generator, deterministic_only: $llm[0].deterministic_only, prompt_mode: $llm[0].prompt_mode, scope_budget: ($llm[0].scope_budget.raw // ""), generated_files: ($llm[0].generated_files | length), intervention_loops: $llmcompare[0].summary.intervention_loops, checks_failed: $llmcompare[0].summary.patchline_checks_failed}, no_facts_generation_proof: {generator: $nofacts[0].generator, deterministic_only: $nofacts[0].deterministic_only, prompt_mode: $nofacts[0].prompt_mode, prompt_hash: $nofacts[0].prompt_hash, output_hash: $nofacts[0].output_hash, generated_files: ($nofacts[0].generated_files | length), intervention_loops: $nofactscompare[0].summary.intervention_loops, checks_failed: $nofactscompare[0].summary.patchline_checks_failed}, fact_grounded_generation_comparison: {fact_grounded_prompt_hash: $llm[0].prompt_hash, without_facts_prompt_hash: $nofacts[0].prompt_hash, fact_grounded_output_hash: $llm[0].output_hash, without_facts_output_hash: $nofacts[0].output_hash, fact_grounded_checks_failed: $llmcompare[0].summary.patchline_checks_failed, without_facts_checks_failed: $nofactscompare[0].summary.patchline_checks_failed, fact_grounded_intervention_loops: $llmcompare[0].summary.intervention_loops, without_facts_intervention_loops: $nofactscompare[0].summary.intervention_loops}, cache_proof: {hit: $cache[0].source.cache_hit, archive_hash: $cache[0].source.archive_hash, resolved_commit: $cache[0].source.resolved_commit}, analyze_proof: {stages: $analyze[0].stages, ci: $analyze[0].ci, commands: ($analyze[0].commands_path // ""), ci_summary: ($analyze[0].ci_artifacts.summary_path // ""), ci_sarif: ($analyze[0].ci_artifacts.sarif_path // ""), redact: $analyze[0].redact, scope_budget: ($analyze[0].summary.scope_budget // ""), ranked_risks: $analyze[0].summary.ranked_risks, generated_files: $analyze[0].summary.generated_files, intervention_loops: $analyze[0].summary.intervention_loops, bundle_files: $bundle_files, hash: $analyze[0].hash}, resume_proof: {resume: $resume[0].resume, reused_stages: ($resume[0].reused_stages | length), ranked_risks: $resume[0].summary.ranked_risks, generated_files: $resume[0].summary.generated_files, intervention_loops: $resume[0].summary.intervention_loops}, structured_proof: {files: $structured[0].files_scanned, field_evidence: ($structured[0].summary_by_category.field_evidence // 0)}}' \
    "$case_out/summary.json" > "$case_out/row.json"
  rows+=("$case_out/row.json")
done <<< "$CASES"

jq -s '{version:"patchline.four-repo-demo/v1", summary: {cases: length, cache_hits: map(select(.performance.cache_hit == true)) | length, cache_hit_rate: ((map(select(.performance.cache_hit == true)) | length) / length)}, cases: .}' "${rows[@]}" > "$OUT/summary.json"

jq -e '
  (.cases | length) >= 4 and
  .summary.cache_hit_rate >= 0 and .summary.cache_hit_rate <= 1 and
  all(.cases[]; .ecosystem != "unknown" and .migration_framework != "unknown" and .repo_size_class != "unknown" and (.available_evidence_types | length) > 0 and .source.owner != null and .source.repo != null and (.source.resolved_commit | test("^[0-9a-f]{40}$")) and (.source.archive_hash | startswith("sha256:")) and (.source.fetched_at | length > 0) and .inventory.files > 0 and .inventory.facts >= .inventory.files and .inventory.schema_evolution > 0 and .inventory.native_commands > 0 and .intake.links > 0 and .intake.time_signals >= 0 and .performance.analyze_runtime_seconds >= 0 and .performance.analyze_max_rss_kb >= 0 and .performance.download_size_bytes > 0 and .performance.maintainer_review_burden > 0 and .sampled_adjudications.false_positive_samples > 0 and .sampled_adjudications.false_negative_samples > 0 and (.sampled_adjudications.notes | all(.note != "" and .adjudication != "")) and .coverage.files_scanned == .inventory.files and .coverage.ranked_risks == .baseline.risks and .coverage.evidence_links == .baseline.evidence_links and .before_after_deltas.baseline_risks == .baseline.risks and .before_after_deltas.generated_files == .compare.generated_files and .before_after_deltas.risks_with_generated_coverage == .compare.risks_with_coverage and .comparisons.grep_only_risk_detection.grep_only_matches == .baseline.grep_only and .comparisons.grep_only_risk_detection.patchline_ranked_risks == .baseline.risks and .comparisons.grep_only_risk_detection.patchline_evidence_links == .baseline.evidence_links and .comparisons.sql_only_without_links.sql_only_ranked_risks == .baseline.sql_only and .comparisons.sql_only_without_links.patchline_ranked_risks == .baseline.risks and .comparisons.sql_only_without_links.patchline_evidence_links == .baseline.evidence_links and .comparisons.identifier_only_without_temporal.identifier_only_links == .baseline.identifier_only_links and .comparisons.identifier_only_without_temporal.patchline_evidence_links == .baseline.evidence_links and .comparisons.identifier_only_without_temporal.temporal_or_date_only_links == .baseline.date_only_links and .comparisons.temporal_only_without_identifiers.temporal_or_date_only_links == .baseline.date_only_links and .comparisons.temporal_only_without_identifiers.patchline_identifier_links == .baseline.identifier_only_links and .comparisons.temporal_only_without_identifiers.patchline_evidence_links == .baseline.evidence_links and .comparisons.deterministic_reanalysis_vs_trust.trusted_without_verification_files == .compare.generated_files and .comparisons.deterministic_reanalysis_vs_trust.trusted_without_verification_checks == 0 and .comparisons.deterministic_reanalysis_vs_trust.deterministic_intervention_loops == .compare.intervention_loops and .comparisons.deterministic_reanalysis_vs_trust.deterministic_checks_failed == .compare.checks_failed and .structured_proof.files > 0 and .structured_proof.field_evidence > 0 and .baseline.risks > 0 and .baseline.code_path_risks > 0 and .baseline.ranking_explanations > 0 and .baseline.provenance_slices > 0 and .baseline.datalog_rows > 0 and .baseline.abstract_effects > 0 and .baseline.symbolic_checks > 0 and .baseline.temporal_windows > 0 and .baseline.recurrences > 0 and .baseline.policy_checks > 0 and .baseline.repair_proof_summaries > 0 and .proposal.files > 0 and .compare.intervention_loops > 0 and .compare.checks_failed == 0 and (.compare.native_checks_failed // 0) == 0 and .llm_command_proof.generator == "llm-command" and .llm_command_proof.deterministic_only == false and .llm_command_proof.prompt_mode == "fact-grounded" and .llm_command_proof.scope_budget != "" and .llm_command_proof.generated_files > 0 and .llm_command_proof.intervention_loops > 0 and .llm_command_proof.checks_failed == 0 and .no_facts_generation_proof.generator == "llm-command" and .no_facts_generation_proof.prompt_mode == "without-facts" and .no_facts_generation_proof.generated_files > 0 and .fact_grounded_generation_comparison.fact_grounded_prompt_hash != .fact_grounded_generation_comparison.without_facts_prompt_hash and .fact_grounded_generation_comparison.fact_grounded_output_hash != .fact_grounded_generation_comparison.without_facts_output_hash and .analyze_proof.ci == true and .analyze_proof.commands != "" and .analyze_proof.ci_summary != "" and .analyze_proof.ci_sarif != "" and .analyze_proof.redact == true and .analyze_proof.scope_budget != "" and .analyze_proof.bundle_files >= 7 and .analyze_proof.ranked_risks > 0 and .analyze_proof.generated_files > 0 and .analyze_proof.intervention_loops > 0 and (.analyze_proof.stages | index("deep")) and .resume_proof.resume == true and .resume_proof.reused_stages >= 5 and .resume_proof.ranked_risks == .analyze_proof.ranked_risks and .resume_proof.generated_files == .analyze_proof.generated_files and .resume_proof.intervention_loops == .analyze_proof.intervention_loops and .cache_proof.hit == true and (.cache_proof.resolved_commit | test("^[0-9a-f]{40}$")))
' "$OUT/summary.json" > /dev/null

jq '{
  version: "patchline.real-repo-slice-matrix/v1",
  generated_from: "real public repository analysis",
  summary,
  slices: [.cases[] | {
    label,
    repo,
    ref,
    subpath,
    ecosystem,
    migration_framework,
    repo_size_class,
    available_evidence_types,
    resolved_commit: .source.resolved_commit,
    archive_hash: .source.archive_hash,
    files_scanned: .inventory.files,
    facts: .inventory.facts,
    schema_evolution: .inventory.schema_evolution,
    native_commands: .inventory.native_commands,
    high_risk_sql: .intake.high_risk,
    linked_candidates: .intake.links,
    time_signals: .intake.time_signals,
    ranked_risks: .baseline.risks,
    generated_artifacts: .proposal.files,
    intervention_loops: .compare.intervention_loops,
    coverage: .coverage,
    before_after_deltas: .before_after_deltas,
    grep_only_comparison: .comparisons.grep_only_risk_detection,
    sql_only_comparison: .comparisons.sql_only_without_links,
    identifier_only_comparison: .comparisons.identifier_only_without_temporal,
    temporal_only_comparison: .comparisons.temporal_only_without_identifiers,
    generation_comparison: .fact_grounded_generation_comparison,
    verification_comparison: .comparisons.deterministic_reanalysis_vs_trust,
    sampled_adjudications,
    performance,
    cache_hit: .cache_proof.hit
  }]
}' "$OUT/summary.json" > "$OUT/slice-matrix.json"

{
  echo "# Patchline four-repo analysis demo"
  echo
  echo "Each row is a real external repository slice fetched, inventoried, converted to facts, checked by intake, and ranked by the baseline stage."
  echo
  echo "| Project slice | Commit | Files | Facts | Schema evolutions | Native commands | Field evidence proof | High-risk SQL | Problems | Baseline risks | Code-path risks | Ranking explanations | Provenance slices | Datalog rows | Abstract effects | Symbolic checks | Temporal windows | Recurrences | Policy checks | Repair proof summaries | Evidence links | Proposal files | Intervention loops | Analyze loops | Bundle files | CI | Redacted | Resume stages | LLM command files | Compare failures | Native checks run | Cache proof |"
  echo "| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- | --- | ---: | ---: | ---: | ---: | --- |"
  jq -r '.cases[] | "| \(.label) (`\(.repo):\(.subpath)`) | `\(.source.resolved_commit[0:12])` | \(.inventory.files) | \(.inventory.facts) | \(.inventory.schema_evolution) | \(.inventory.native_commands) | \(.structured_proof.field_evidence) | \(.intake.high_risk) | \(.intake.problems) | \(.baseline.risks) | \(.baseline.code_path_risks) | \(.baseline.ranking_explanations) | \(.baseline.provenance_slices) | \(.baseline.datalog_rows) | \(.baseline.abstract_effects) | \(.baseline.symbolic_checks) | \(.baseline.temporal_windows) | \(.baseline.recurrences) | \(.baseline.policy_checks) | \(.baseline.repair_proof_summaries) | \(.baseline.evidence_links) | \(.proposal.files) | \(.compare.intervention_loops) | \(.analyze_proof.intervention_loops) | \(.analyze_proof.bundle_files) | \(.analyze_proof.ci) | \(.analyze_proof.redact) | \(.resume_proof.reused_stages) | \(.llm_command_proof.generated_files) | \(.compare.checks_failed) | \(.compare.native_checks_run) | \(.cache_proof.hit) |"' "$OUT/summary.json"
  echo
  echo 'Each case directory contains fetch metadata, inventory outputs, `facts.jsonl`, `project-map.md`, intake outputs, baseline JSON/Markdown/SARIF, isolated proposal artifacts, and compare reports.'
} > "$OUT/summary.md"

{
  echo "# Real repo slice matrix"
  echo
  echo "This matrix is generated from public repository slices that were fetched and analyzed by Patchline."
  echo
  jq -r '"Cache hit rate: \(.summary.cache_hit_rate) (\(.summary.cache_hits)/\(.summary.cases))"' "$OUT/slice-matrix.json"
  echo
  echo "| Project slice | Ecosystem | Migration framework | Size class | Evidence types | Commit | Runtime s | Max RSS KB | Download bytes | Review burden | Files | Facts | Grep-only matches | SQL-only risks | Identifier-only links | Temporal/date-only links | Ranked risks | Risk delta vs grep | Risk delta vs SQL-only | Evidence-link delta vs identifier-only | Evidence-link delta vs temporal-only | Identifier-link delta vs temporal-only | Links | Time signals | Generated artifacts | FP samples | FN samples | Fact prompt differs | Fact output differs | Trusted checks | Reanalysis loops | Reanalysis failures | Covered risks | New high-risk SQL | Cache hit |"
  echo "| --- | --- | --- | --- | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- | --- | ---: | ---: | ---: | ---: | ---: | --- |"
  jq -r '.slices[] | "| \(.label) (`\(.repo):\(.subpath)`) | \(.ecosystem) | \(.migration_framework) | \(.repo_size_class) | \(.available_evidence_types | join(", ")) | `\(.resolved_commit[0:12])` | \(.performance.analyze_runtime_seconds) | \(.performance.analyze_max_rss_kb) | \(.performance.download_size_bytes) | \(.performance.maintainer_review_burden) | \(.files_scanned) | \(.facts) | \(.grep_only_comparison.grep_only_matches) | \(.sql_only_comparison.sql_only_ranked_risks) | \(.identifier_only_comparison.identifier_only_links) | \(.identifier_only_comparison.temporal_or_date_only_links) | \(.ranked_risks) | \(.grep_only_comparison.ranked_risk_delta) | \(.sql_only_comparison.ranked_risk_delta) | \(.identifier_only_comparison.evidence_link_delta) | \(.temporal_only_comparison.evidence_link_delta) | \(.temporal_only_comparison.identifier_link_delta) | \(.linked_candidates) | \(.time_signals) | \(.generated_artifacts) | \(.sampled_adjudications.false_positive_samples) | \(.sampled_adjudications.false_negative_samples) | \(.generation_comparison.fact_grounded_prompt_hash != .generation_comparison.without_facts_prompt_hash) | \(.generation_comparison.fact_grounded_output_hash != .generation_comparison.without_facts_output_hash) | \(.verification_comparison.trusted_without_verification_checks) | \(.verification_comparison.deterministic_intervention_loops) | \(.verification_comparison.deterministic_checks_failed) | \(.before_after_deltas.risks_with_generated_coverage) | \(.before_after_deltas.new_high_risk_sql) | \(.cache_hit) |"' "$OUT/slice-matrix.json"
  echo
  echo "## Sampled adjudication notes"
  echo
  jq -r '.slices[] | . as $slice | .sampled_adjudications.notes[] | "- **\($slice.label)** `\(.sample_type)`: \(.finding) — \(.note)"' "$OUT/slice-matrix.json"
} > "$OUT/slice-matrix.md"

echo "four-repo analysis summary: $OUT/summary.md"
echo "real repo slice matrix: $OUT/slice-matrix.md"
