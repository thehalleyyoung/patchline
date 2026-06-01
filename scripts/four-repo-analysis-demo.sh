#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

OUT="${1:-results/generated/four-repo-analysis-demo}"
CASES="${PATCHLINE_FOUR_REPO_CASES:-Django auth migrations|django/django|django/contrib/auth/migrations
Bytebase migrations|bytebase/bytebase|backend/migrator/migration
Mastodon migrations|mastodon/mastodon|db/migrate
Discourse migrations|discourse/discourse|db/migrate}"

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

  jq \
    --arg label "$label" \
    --arg repo "$repo" \
    --arg subpath "$subpath" \
    '. + {label: $label, repo: $repo, subpath: $subpath, case_dir: "'"$case_out"'"}' \
    "$case_out/summary.json" > "$case_out/row.json"
  rows+=("$case_out/row.json")
done <<< "$CASES"

jq -s '{version:"patchline.four-repo-demo/v1", cases: .}' "${rows[@]}" > "$OUT/summary.json"

jq -e '
  (.cases | length) >= 4 and
  all(.cases[]; .inventory.files > 0 and .inventory.facts >= .inventory.files and .baseline.risks > 0)
' "$OUT/summary.json" > /dev/null

{
  echo "# Patchline four-repo analysis demo"
  echo
  echo "Each row is a real external repository slice fetched, inventoried, converted to facts, checked by intake, and ranked by the baseline stage."
  echo
  echo "| Project slice | Files | Facts | High-risk SQL | Problems | Baseline risks | Evidence links |"
  echo "| --- | ---: | ---: | ---: | ---: | ---: | ---: |"
  jq -r '.cases[] | "| \(.label) (`\(.repo):\(.subpath)`) | \(.inventory.files) | \(.inventory.facts) | \(.intake.high_risk) | \(.intake.problems) | \(.baseline.risks) | \(.baseline.evidence_links) |"' "$OUT/summary.json"
  echo
  echo 'Each case directory contains fetch metadata, inventory outputs, `facts.jsonl`, `project-map.md`, intake outputs, and baseline JSON/Markdown/SARIF.'
} > "$OUT/summary.md"

echo "four-repo analysis summary: $OUT/summary.md"
