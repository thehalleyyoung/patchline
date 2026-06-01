#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

OUT="${1:-results/generated/repo-analysis-demo}"
REPO="${PATCHLINE_REPO_DEMO_REPO:-django/django}"
SUBPATH="${PATCHLINE_REPO_DEMO_SUBPATH:-django/contrib/auth/migrations}"

rm -rf "$OUT"
mkdir -p "$OUT"

FETCH_OUT="$OUT/fetched"
INVENTORY_OUT="$OUT/inventory"
INTAKE_OUT="$OUT/intake"
BASELINE_OUT="$OUT/baseline"
PROPOSAL_OUT="$OUT/proposal"
COMPARE_OUT="$OUT/compare"

go run ./cmd/patchline repo fetch "$REPO" --subpath "$SUBPATH" --out "$FETCH_OUT" --json > "$OUT/fetch.json"
SCAN_ROOT="$(jq -r '.source.scanned_root' "$OUT/fetch.json")"

go run ./cmd/patchline repo inventory "$SCAN_ROOT" --out "$INVENTORY_OUT" --json > "$OUT/inventory.json"
go run ./cmd/patchline intake "$SCAN_ROOT" --out "$INTAKE_OUT" --json > "$OUT/intake.json"
go run ./cmd/patchline repo baseline --inventory "$INVENTORY_OUT" --intake "$INTAKE_OUT" --out "$BASELINE_OUT" --json > "$OUT/baseline.json"
go run ./cmd/patchline repo propose --from-report "$BASELINE_OUT" --kind all --out "$PROPOSAL_OUT" --json > "$OUT/proposal.json"
go run ./cmd/patchline repo compare --before "$BASELINE_OUT" --after "$PROPOSAL_OUT" --out "$COMPARE_OUT" --json > "$OUT/compare.json"
FACT_COUNT="$(wc -l < "$INVENTORY_OUT/facts.jsonl" | tr -d ' ')"

jq -n \
  --slurpfile fetch "$OUT/fetch.json" \
  --slurpfile inventory "$OUT/inventory.json" \
  --slurpfile intake "$OUT/intake.json" \
  --slurpfile baseline "$OUT/baseline.json" \
  --slurpfile proposal "$OUT/proposal.json" \
  --slurpfile compare "$OUT/compare.json" \
  --argjson fact_count "$FACT_COUNT" \
  '{
    version: "patchline.repo-demo/v1",
    repo: $fetch[0].source.input,
    subpath: $fetch[0].source.subpath,
    source: $fetch[0].source,
    inventory: {
      files: $inventory[0].files_scanned,
      languages: $inventory[0].languages,
      frameworks: $inventory[0].frameworks,
      migration_roots: $inventory[0].migration_roots,
      schema_evolution: ($inventory[0].summary_by_category.schema_evolution // 0),
      native_commands: ($inventory[0].summary_by_category.native_commands // 0),
      field_evidence: ($inventory[0].summary_by_category.field_evidence // 0),
      facts: $fact_count,
      next_commands: $inventory[0].next_commands
    },
    intake: {
      files: $intake[0].summary.files_scanned,
      loose_sql: $intake[0].summary.loose_sql_snippets,
      high_risk: $intake[0].summary.high_risk_sql_statements,
      problems: $intake[0].summary.problem_candidates,
      causes: $intake[0].summary.cause_candidates,
      links: $intake[0].summary.linked_candidates
    },
    baseline: {
      risks: $baseline[0].summary.ranked_risks,
      code_path_risks: ($baseline[0].summary.code_path_ranked_risks // 0),
      evidence_links: $baseline[0].summary.evidence_links,
      grep_only: $baseline[0].summary.grep_only_matches,
      sql_only: $baseline[0].summary.sql_only_ranked_risks,
      identifier_only_links: $baseline[0].summary.identifier_only_links
    },
    proposal: {
      kind: $proposal[0].kind,
      generator: $proposal[0].generator,
      trust: $proposal[0].trust,
      files: ($proposal[0].generated_files | length),
      output_hash: $proposal[0].output_hash
    },
    compare: {
      generated_files: $compare[0].summary.generated_files,
      risks_with_coverage: $compare[0].summary.risks_with_coverage,
      checks_passed: $compare[0].summary.patchline_checks_passed,
      checks_failed: $compare[0].summary.patchline_checks_failed,
      native_checks_run: ($compare[0].summary.native_checks_run // 0),
      native_checks_passed: ($compare[0].summary.native_checks_passed // 0),
      native_checks_failed: ($compare[0].summary.native_checks_failed // 0),
      native_checks_skipped: ($compare[0].summary.native_checks_skipped // 0),
      rejected: $compare[0].summary.rejected
    }
  }' > "$OUT/summary.json"

{
  echo "# Patchline repo analysis demo"
  echo
  echo "Real GitHub repo: \`$REPO:$SUBPATH\`"
  echo
  echo "| stage | output |"
  echo "| --- | --- |"
  echo "| fetch metadata | \`$OUT/fetch.json\` |"
  echo "| inventory | \`$INVENTORY_OUT/inventory.md\` |"
  echo "| project map | \`$INVENTORY_OUT/project-map.md\` |"
  echo "| fact stream | \`$INVENTORY_OUT/facts.jsonl\` |"
  echo "| intake | \`$INTAKE_OUT/summary.md\` |"
  echo "| baseline | \`$BASELINE_OUT/baseline.md\` |"
  echo "| baseline SARIF | \`$BASELINE_OUT/baseline.sarif\` |"
  echo "| proposal | \`$PROPOSAL_OUT/proposal.md\` |"
  echo "| proposal patch | \`$PROPOSAL_OUT/proposal.patch\` |"
  echo "| compare | \`$COMPARE_OUT/compare.md\` |"
  echo
  echo "## Summary"
  echo
  jq -r '"- files inventoried: \(.inventory.files)\n- project facts: \(.inventory.facts)\n- schema evolution findings: \(.inventory.schema_evolution)\n- native commands: \(.inventory.native_commands)\n- field evidence: \(.inventory.field_evidence)\n- intake high-risk SQL: \(.intake.high_risk)\n- problem candidates: \(.intake.problems)\n- candidate links: \(.intake.links)\n- baseline ranked risks: \(.baseline.risks)\n- baseline code-path risks: \(.baseline.code_path_risks)\n- baseline evidence links: \(.baseline.evidence_links)\n- generated proposal files: \(.proposal.files)\n- proposal output hash: \(.proposal.output_hash)\n- compare checks passed: \(.compare.checks_passed)\n- compare checks failed: \(.compare.checks_failed)\n- native checks run: \(.compare.native_checks_run)\n- native checks passed: \(.compare.native_checks_passed)\n- native checks failed: \(.compare.native_checks_failed)\n- native checks skipped: \(.compare.native_checks_skipped)"' "$OUT/summary.json"
} > "$OUT/summary.md"

echo "repo analysis summary: $OUT/summary.md"
