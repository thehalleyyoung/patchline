#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

OUT="${1:-results/generated/plug-and-play-demo}"
CASES="${PATCHLINE_PLUGPLAY_CASES:-Bytebase migrations|bytebase/bytebase|backend/migrator/migration
Django auth migrations|django/django|django/contrib/auth/migrations
Mastodon migrations|mastodon/mastodon|db/migrate}"

rm -rf "$OUT"
mkdir -p "$OUT/cases"

rows=()
while IFS='|' read -r label repo subpath; do
  [[ -z "${label// }" ]] && continue
  slug="$(printf '%s-%s' "$repo" "$subpath" | tr '/[:space:]' '--' | tr -cd '[:alnum:]_.-')"
  case_out="$OUT/cases/$slug"
  mkdir -p "$case_out"

  echo "==> $label ($repo:$subpath)"
  go run ./cmd/patchline intake --github "$repo" --subpath "$subpath" --out "$case_out" | tee "$case_out/run.log"

  row="$case_out/row.json"
  jq \
    --arg label "$label" \
    --arg repo "$repo" \
    --arg subpath "$subpath" \
    '{
      label: $label,
      repo: $repo,
      subpath: $subpath,
      hash: .hash,
      files: .summary.files_scanned,
      sql_files: .summary.sql_files,
      loose_sql: .summary.loose_sql_snippets,
      high_risk: .summary.high_risk_sql_statements,
      medium_risk: .summary.medium_risk_sql_statements,
      problems: .summary.problem_candidates,
      causes: .summary.cause_candidates,
      repair_candidates: .summary.repair_candidates,
      links: .summary.linked_candidates,
      sample_problem: (.problem_candidates[0] // null),
      sample_repair: (.repair_candidates[0] // null)
    }' "$case_out/summary.json" > "$row"
  rows+=("$row")
done <<< "$CASES"

jq -s '{version:"patchline.plug-play-demo/v1", cases: .}' "${rows[@]}" > "$OUT/summary.json"

{
  echo "# Patchline plug-and-play demo"
  echo
  echo "These are real public GitHub project subpaths scanned with \`patchline intake --github ... --subpath ...\`."
  echo
  echo "| Project slice | Files | SQL files | Loose SQL | High-risk SQL | Problems | Causes | Repairs | Links |"
  echo "| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |"
  jq -r '.cases[] | "| \(.label) (`\(.repo):\(.subpath)`) | \(.files) | \(.sql_files) | \(.loose_sql) | \(.high_risk) | \(.problems) | \(.causes) | \(.repair_candidates) | \(.links) |"' "$OUT/summary.json"
  echo
  echo "## Example findings"
  echo
  jq -r '.cases[] | select(.sample_problem != null) | "- **\(.label)**: `\(.sample_problem.path)` table=`\(.sample_problem.table // "")` — \(.sample_problem.rationale)"' "$OUT/summary.json"
  echo
  echo "Each case directory contains the full intake \`summary.json\`, \`summary.md\`, and command log."
} > "$OUT/summary.md"

echo "plug-and-play summary: $OUT/summary.md"
