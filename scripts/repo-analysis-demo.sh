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

go run ./cmd/patchline repo fetch "$REPO" --subpath "$SUBPATH" --out "$FETCH_OUT" --json > "$OUT/fetch.json"
SCAN_ROOT="$(jq -r '.source.scanned_root' "$OUT/fetch.json")"

go run ./cmd/patchline repo inventory "$SCAN_ROOT" --out "$INVENTORY_OUT" --json > "$OUT/inventory.json"
go run ./cmd/patchline intake "$SCAN_ROOT" --out "$INTAKE_OUT" --json > "$OUT/intake.json"
FACT_COUNT="$(wc -l < "$INVENTORY_OUT/facts.jsonl" | tr -d ' ')"

jq -n \
  --slurpfile fetch "$OUT/fetch.json" \
  --slurpfile inventory "$OUT/inventory.json" \
  --slurpfile intake "$OUT/intake.json" \
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
  echo
  echo "## Summary"
  echo
  jq -r '"- files inventoried: \(.inventory.files)\n- project facts: \(.inventory.facts)\n- intake high-risk SQL: \(.intake.high_risk)\n- problem candidates: \(.intake.problems)\n- candidate links: \(.intake.links)"' "$OUT/summary.json"
} > "$OUT/summary.md"

echo "repo analysis summary: $OUT/summary.md"
