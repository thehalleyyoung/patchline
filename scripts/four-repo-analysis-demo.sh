#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

OUT="${1:-results/generated/four-repo-analysis-demo}"
CASES="${PATCHLINE_FOUR_REPO_CASES:-Forem migrations|forem/forem|db/migrate
Bytebase migrations|bytebase/bytebase|backend/migrator/migration
Mastodon migrations|mastodon/mastodon|db/migrate
Lobsters migrations|lobsters/lobsters|db/migrate}"

rm -rf "$OUT"
mkdir -p "$OUT/cases"

rows=()
while IFS='|' read -r label repo subpath; do
  [[ -z "${label// }" ]] && continue
  slug="$(printf '%s-%s' "$repo" "$subpath" | tr '/[:space:]' '--' | tr -cd '[:alnum:]_.-')"
  case_out="$OUT/cases/$slug"

  echo "==> $label ($repo:$subpath)"
  PATCHLINE_REPO_DEMO_REPO="$repo" \
    PATCHLINE_REPO_DEMO_SUBPATH="$subpath" \
    bash scripts/repo-analysis-demo.sh "$case_out"
  go run ./cmd/patchline repo propose --from-report "$case_out/baseline" --proposal-kind tests --budget files=1,lines=120,tokens=4000,changes=3 --llm-command cat --out "$case_out/llm-proposal" --json > "$case_out/llm-proposal.json"
  go run ./cmd/patchline repo compare --before "$case_out/baseline" --after "$case_out/llm-proposal" --out "$case_out/llm-compare" --json > "$case_out/llm-compare.json"
  go run ./cmd/patchline repo fetch "$repo" --subpath "$subpath" --out "$case_out/cache-proof" --json > "$case_out/cache-proof.json"
  go run ./cmd/patchline repo analyze --github "$repo" --subpath "$subpath" --stages inventory,baseline,propose,compare,deep --proposal-kind all --budget files=15,lines=120,tokens=50000,changes=3 --ci --redact --no-llm --out "$case_out/analyze" --json > "$case_out/analyze.json"
  jq -e '.input | startswith("[redacted:")' "$case_out/analyze/analysis-bundle/source.json" > /dev/null
  test -s "$case_out/analyze/ci/summary.md"
  grep -q "upload-sarif" "$case_out/analyze/ci/github-actions-upload.yml"
  grep -q "upload-artifact" "$case_out/analyze/ci/github-actions-upload.yml"
  go run ./cmd/patchline repo analyze --github "$repo" --subpath "$subpath" --stages inventory,baseline,propose,compare,deep --proposal-kind all --budget files=15,lines=120,tokens=50000,changes=3 --ci --redact --no-llm --resume --out "$case_out/analyze" --json > "$case_out/analyze-resume.json"
  bundle_files="$(find "$case_out/analyze/analysis-bundle" -maxdepth 1 -type f | wc -l | tr -d ' ')"
  for required in source.json facts.jsonl baseline.json proposal.patch compare.json summary.md summary.sarif; do
    test -s "$case_out/analyze/analysis-bundle/$required"
  done
  go run ./cmd/patchline repo fetch "$repo" --subpath ".github/workflows" --out "$case_out/structured-fetch" --json > "$case_out/structured-fetch.json"
  structured_root="$(jq -r '.source.scanned_root' "$case_out/structured-fetch.json")"
  go run ./cmd/patchline repo inventory "$structured_root" --out "$case_out/structured-inventory" --json > "$case_out/structured-inventory.json"

  jq \
    --arg label "$label" \
    --arg repo "$repo" \
    --arg subpath "$subpath" \
    --argjson bundle_files "$bundle_files" \
    --slurpfile llm "$case_out/llm-proposal.json" \
    --slurpfile llmcompare "$case_out/llm-compare.json" \
    --slurpfile cache "$case_out/cache-proof.json" \
    --slurpfile analyze "$case_out/analyze.json" \
    --slurpfile resume "$case_out/analyze-resume.json" \
    --slurpfile structured "$case_out/structured-inventory.json" \
    '. + {label: $label, repo: $repo, subpath: $subpath, case_dir: "'"$case_out"'", llm_command_proof: {generator: $llm[0].generator, deterministic_only: $llm[0].deterministic_only, scope_budget: ($llm[0].scope_budget.raw // ""), generated_files: ($llm[0].generated_files | length), intervention_loops: $llmcompare[0].summary.intervention_loops, checks_failed: $llmcompare[0].summary.patchline_checks_failed}, cache_proof: {hit: $cache[0].source.cache_hit, archive_hash: $cache[0].source.archive_hash, resolved_commit: $cache[0].source.resolved_commit}, analyze_proof: {stages: $analyze[0].stages, ci: $analyze[0].ci, ci_summary: ($analyze[0].ci_artifacts.summary_path // ""), ci_sarif: ($analyze[0].ci_artifacts.sarif_path // ""), redact: $analyze[0].redact, scope_budget: ($analyze[0].summary.scope_budget // ""), ranked_risks: $analyze[0].summary.ranked_risks, generated_files: $analyze[0].summary.generated_files, intervention_loops: $analyze[0].summary.intervention_loops, bundle_files: $bundle_files, hash: $analyze[0].hash}, resume_proof: {resume: $resume[0].resume, reused_stages: ($resume[0].reused_stages | length), ranked_risks: $resume[0].summary.ranked_risks, generated_files: $resume[0].summary.generated_files, intervention_loops: $resume[0].summary.intervention_loops}, structured_proof: {files: $structured[0].files_scanned, field_evidence: ($structured[0].summary_by_category.field_evidence // 0)}}' \
    "$case_out/summary.json" > "$case_out/row.json"
  rows+=("$case_out/row.json")
done <<< "$CASES"

jq -s '{version:"patchline.four-repo-demo/v1", cases: .}' "${rows[@]}" > "$OUT/summary.json"

jq -e '
  (.cases | length) >= 4 and
  all(.cases[]; .source.owner != null and .source.repo != null and (.source.resolved_commit | test("^[0-9a-f]{40}$")) and (.source.archive_hash | startswith("sha256:")) and (.source.fetched_at | length > 0) and .inventory.files > 0 and .inventory.facts >= .inventory.files and .inventory.schema_evolution > 0 and .inventory.native_commands > 0 and .structured_proof.files > 0 and .structured_proof.field_evidence > 0 and .baseline.risks > 0 and .baseline.code_path_risks > 0 and .baseline.ranking_explanations > 0 and .baseline.provenance_slices > 0 and .baseline.datalog_rows > 0 and .baseline.abstract_effects > 0 and .baseline.symbolic_checks > 0 and .baseline.temporal_windows > 0 and .baseline.recurrences > 0 and .baseline.policy_checks > 0 and .baseline.repair_proof_summaries > 0 and .proposal.files > 0 and .compare.intervention_loops > 0 and .compare.checks_failed == 0 and (.compare.native_checks_failed // 0) == 0 and .llm_command_proof.generator == "llm-command" and .llm_command_proof.deterministic_only == false and .llm_command_proof.scope_budget != "" and .llm_command_proof.generated_files > 0 and .llm_command_proof.intervention_loops > 0 and .llm_command_proof.checks_failed == 0 and .analyze_proof.ci == true and .analyze_proof.ci_summary != "" and .analyze_proof.ci_sarif != "" and .analyze_proof.redact == true and .analyze_proof.scope_budget != "" and .analyze_proof.bundle_files >= 7 and .analyze_proof.ranked_risks > 0 and .analyze_proof.generated_files > 0 and .analyze_proof.intervention_loops > 0 and (.analyze_proof.stages | index("deep")) and .resume_proof.resume == true and .resume_proof.reused_stages >= 5 and .resume_proof.ranked_risks == .analyze_proof.ranked_risks and .resume_proof.generated_files == .analyze_proof.generated_files and .resume_proof.intervention_loops == .analyze_proof.intervention_loops and .cache_proof.hit == true and (.cache_proof.resolved_commit | test("^[0-9a-f]{40}$")))
' "$OUT/summary.json" > /dev/null

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

echo "four-repo analysis summary: $OUT/summary.md"
