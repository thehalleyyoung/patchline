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
  go run ./cmd/patchline repo fetch "$repo" --subpath "$subpath" --out "$case_out/cache-proof" --json > "$case_out/cache-proof.json"

  jq \
    --arg label "$label" \
    --arg repo "$repo" \
    --arg subpath "$subpath" \
    --slurpfile cache "$case_out/cache-proof.json" \
    '. + {label: $label, repo: $repo, subpath: $subpath, case_dir: "'"$case_out"'", cache_proof: {hit: $cache[0].source.cache_hit, archive_hash: $cache[0].source.archive_hash, resolved_commit: $cache[0].source.resolved_commit}}' \
    "$case_out/summary.json" > "$case_out/row.json"
  rows+=("$case_out/row.json")
done <<< "$CASES"

jq -s '{version:"patchline.four-repo-demo/v1", cases: .}' "${rows[@]}" > "$OUT/summary.json"

jq -e '
  (.cases | length) >= 4 and
  all(.cases[]; .source.owner != null and .source.repo != null and (.source.resolved_commit | test("^[0-9a-f]{40}$")) and (.source.archive_hash | startswith("sha256:")) and (.source.fetched_at | length > 0) and .inventory.files > 0 and .inventory.facts >= .inventory.files and .baseline.risks > 0 and .proposal.files > 0 and .compare.checks_failed == 0 and .cache_proof.hit == true and (.cache_proof.resolved_commit | test("^[0-9a-f]{40}$")))
' "$OUT/summary.json" > /dev/null

{
  echo "# Patchline four-repo analysis demo"
  echo
  echo "Each row is a real external repository slice fetched, inventoried, converted to facts, checked by intake, and ranked by the baseline stage."
  echo
  echo "| Project slice | Commit | Files | Facts | High-risk SQL | Problems | Baseline risks | Evidence links | Proposal files | Compare failures | Cache proof |"
  echo "| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |"
  jq -r '.cases[] | "| \(.label) (`\(.repo):\(.subpath)`) | `\(.source.resolved_commit[0:12])` | \(.inventory.files) | \(.inventory.facts) | \(.intake.high_risk) | \(.intake.problems) | \(.baseline.risks) | \(.baseline.evidence_links) | \(.proposal.files) | \(.compare.checks_failed) | \(.cache_proof.hit) |"' "$OUT/summary.json"
  echo
  echo 'Each case directory contains fetch metadata, inventory outputs, `facts.jsonl`, `project-map.md`, intake outputs, baseline JSON/Markdown/SARIF, isolated proposal artifacts, and compare reports.'
} > "$OUT/summary.md"

echo "four-repo analysis summary: $OUT/summary.md"
