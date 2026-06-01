#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

CATALOG="${1:-examples/real-repo-catalog.json}"
OUT="${2:-results/generated/real-repo-catalog-gate}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.real-repo-catalog/v1" and
  (.slices | length) >= 25 and
  ([.slices[].id] | unique | length) == (.slices | length) and
  all(.slices[];
    .source_host == "github" and
    (.repo | contains("/")) and
    (.ref | test("^[0-9a-f]{40}$")) and
    (.subpath | length) > 0 and
    (.ecosystem | length) > 0 and
    (.migration_framework | length) > 0 and
    (.available_evidence_types | length) > 0
  )
' "$CATALOG" > /dev/null

for required in \
  "Rails Active Record" "Django migrations" "Alembic" "Flyway" "Liquibase" "Prisma" "TypeORM" "EF Core" "Go SQL migrations" "golang-migrate" "Bytebase migrator SQL" "Knex-style migrations"; do
  jq -e --arg required "$required" '[.slices[].migration_framework] | any(. == $required)' "$CATALOG" > /dev/null
done

rows=()
while IFS=$'\t' read -r id repo ref subpath ecosystem framework monorepo; do
  case_out="$OUT/$id"
  mkdir -p "$case_out"
  code="000"
  if command -v gh >/dev/null 2>&1 && gh api "repos/$repo/contents/$subpath?ref=$ref" > "$case_out/contents.json" 2> "$case_out/gh.err"; then
    code="200"
  else
    code="$(curl -L -sS -o "$case_out/contents.json" -w '%{http_code}' "https://api.github.com/repos/$repo/contents/$subpath?ref=$ref")"
  fi
  if [ "$code" != "200" ]; then
    echo "catalog path failed: $id $repo $ref $subpath status=$code" >&2
    exit 1
  fi
  entries="$(jq 'length' "$case_out/contents.json")"
  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg ref "$ref" \
    --arg subpath "$subpath" \
    --arg ecosystem "$ecosystem" \
    --arg framework "$framework" \
    --arg monorepo "$monorepo" \
    --argjson entries "$entries" \
    '{
      id: $id,
      repo: $repo,
      pinned_commit: $ref,
      subpath: $subpath,
      ecosystem: $ecosystem,
      migration_framework: $framework,
      monorepo: ($monorepo == "true"),
      path_entries: $entries,
      verified: true
    }' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.slices[] | [.id, .repo, .ref, .subpath, .ecosystem, .migration_framework, (.monorepo | tostring)] | @tsv' "$CATALOG")

jq -s '{
  version:"patchline.real-repo-catalog-gate-results/v1",
  slice_count: length,
  ecosystems: ([.[].ecosystem] | unique),
  frameworks: ([.[].migration_framework] | unique),
  monorepos: ([.[] | select(.monorepo == true)] | length),
  slices: .
}' "${rows[@]}" > "$OUT/summary.json"

jq -e '
  .slice_count >= 25 and
  (.ecosystems | length) >= 7 and
  (.frameworks | length) >= 12 and
  .monorepos >= 8 and
  all(.slices[]; .verified == true and .path_entries > 0)
' "$OUT/summary.json" > /dev/null

echo "real-repo catalog gate passed: $(jq '.slice_count' "$OUT/summary.json") pinned public slices verified"
